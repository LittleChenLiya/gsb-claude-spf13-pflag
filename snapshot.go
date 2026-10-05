package pflag

import (
	"fmt"
	"reflect"
	"sync"
)

// ChangeSource identifies how the current value of a flag came to differ
// from its registered default value.
type ChangeSource int

const (
	// ChangeSourceNone means the flag has not been changed; it still holds
	// its registered default value.
	ChangeSourceNone ChangeSource = iota
	// ChangeSourceParse means the flag was first changed while parsing
	// command-line arguments (FlagSet.Parse).
	ChangeSourceParse
	// ChangeSourceSet means the flag was first changed by an explicit
	// caller-driven FlagSet.Set call.
	ChangeSourceSet
)

// String returns a human-readable name for the change source.
func (s ChangeSource) String() string {
	switch s {
	case ChangeSourceNone:
		return "none"
	case ChangeSourceParse:
		return "parse"
	case ChangeSourceSet:
		return "set"
	}
	return fmt.Sprintf("ChangeSource(%d)", int(s))
}

// FlagSnapshot is an explicit snapshot of a flag default value, captured
// at registration time, together with a query-time report of how the
// current value differs from that default.
//
// Default holds an independent copy of the registered default value:
//   - for slice flags (values implementing SliceValue) it is a []string
//     copied from GetSlice at registration time, so later mutation of the
//     original default slice does not affect the snapshot;
//   - for built-in scalar flags it is the immutable default string (the
//     same text as DefValue);
//   - for custom Value types it is whatever the registered SnapshotterFunc
//     or the ValueSnapshotter implementation of the value returned.
//
// Current, Changed and Source are filled in at query time. Querying a
// snapshot never modifies Flag.Changed, the flag ordering, or any other
// FlagSet state.
type FlagSnapshot struct {
	Name     string       // normalized flag name
	DefValue string       // default value text as captured at registration
	Default  interface{}  // independent copy of the registered default value
	Current  string       // current value text at query time
	Changed  bool         // whether the flag was changed, at query time
	Source   ChangeSource // how the flag was first changed (none/parse/set)
}

// ValueSnapshotter may be implemented by custom Value types to provide an
// independent copy of their value for default snapshots. It is consulted
// before the SliceValue fallback but after any SnapshotterFunc registered
// for the concrete type via RegisterValueSnapshotter.
type ValueSnapshotter interface {
	// SnapshotValue returns an independent copy of the current value.
	// It is called once, at flag registration time.
	SnapshotValue() (interface{}, error)
}

// SnapshotterFunc creates an independent copy of a flag value for a
// default snapshot. It is called once, at flag registration time.
type SnapshotterFunc func(Value) (interface{}, error)

// SnapshotUnsupportedError is recorded in a flag snapshot record when no
// snapshot method is known for the flag Value type: the type is not a
// built-in pflag type, does not implement SliceValue or ValueSnapshotter,
// and no SnapshotterFunc was registered for it. pflag deliberately does
// not fall back to guessing a custom type snapshot from its String()
// output, because String is not guaranteed to round-trip or to be
// immutable for arbitrary custom values.
type SnapshotUnsupportedError struct {
	// FlagName is the name of the flag whose default could not be snapshotted.
	FlagName string
	// ValueType is the concrete type of the flag Value.
	ValueType string
}

func (e *SnapshotUnsupportedError) Error() string {
	return fmt.Sprintf("pflag: no snapshotter registered for flag %q value type %s; "+
		"implement pflag.ValueSnapshotter or call pflag.RegisterValueSnapshotter",
		e.FlagName, e.ValueType)
}

var (
	snapshottersMu sync.RWMutex
	// snapshotters maps a concrete Value type to its snapshot function.
	// Custom Value types must be registered here (or implement
	// ValueSnapshotter) to be snapshot-able.
	snapshotters = map[reflect.Type]SnapshotterFunc{}
	// builtinScalarTypes holds the concrete types of pflag built-in scalar
	// values, whose defaults are fully and immutably represented by their
	// String() form.
	builtinScalarTypes = map[reflect.Type]bool{}
)

func init() {
	for _, v := range []Value{
		(*boolValue)(nil),
		(*boolfuncValue)(nil),
		(*bytesHexValue)(nil),
		(*bytesBase64Value)(nil),
		(*countValue)(nil),
		(*durationValue)(nil),
		(*float32Value)(nil),
		(*float64Value)(nil),
		(*funcValue)(nil),
		(*intValue)(nil),
		(*int8Value)(nil),
		(*int16Value)(nil),
		(*int32Value)(nil),
		(*int64Value)(nil),
		(*ipValue)(nil),
		(*ipMaskValue)(nil),
		(*ipNetValue)(nil),
		(*ipNetSliceValue)(nil),
		(*stringValue)(nil),
		(*stringToIntValue)(nil),
		(*stringToInt64Value)(nil),
		(*stringToStringValue)(nil),
		(*textValue)(nil),
		(*timeValue)(nil),
		(*uintValue)(nil),
		(*uint16Value)(nil),
		(*uint32Value)(nil),
		(*uint64Value)(nil),
		(*uint8Value)(nil),
	} {
		builtinScalarTypes[reflect.TypeOf(v)] = true
	}
}

// RegisterValueSnapshotter registers a snapshot function for a custom
// Value type. sample is a (possibly nil pointer) sample of the concrete
// Value type, used only to identify the type, e.g.
//
//	pflag.RegisterValueSnapshotter((*myValue)(nil), func(v pflag.Value) (interface{}, error) {
//		return v.(*myValue).clone(), nil
//	})
//
// It panics if sample or fn is nil. Registration is global to the process
// and may be called at any time before flags of that type are defined.
func RegisterValueSnapshotter(sample Value, fn SnapshotterFunc) {
	if sample == nil || reflect.TypeOf(sample) == nil {
		panic("pflag: RegisterValueSnapshotter requires a non-nil sample Value")
	}
	if fn == nil {
		panic("pflag: RegisterValueSnapshotter requires a non-nil SnapshotterFunc")
	}
	t := reflect.TypeOf(sample)
	snapshottersMu.Lock()
	defer snapshottersMu.Unlock()
	snapshotters[t] = fn
}

// snapshotValue captures an independent copy of the current value of v.
func snapshotValue(v Value) (interface{}, error) {
	t := reflect.TypeOf(v)

	snapshottersMu.RLock()
	fn, ok := snapshotters[t]
	snapshottersMu.RUnlock()
	if ok {
		return fn(v)
	}

	if vs, ok := v.(ValueSnapshotter); ok {
		return vs.SnapshotValue()
	}

	if sv, ok := v.(SliceValue); ok {
		// Slice defaults must be copied so that later mutation of the
		// original default slice does not leak into the snapshot.
		cur := sv.GetSlice()
		cp := make([]string, len(cur))
		copy(cp, cur)
		return cp, nil
	}

	if builtinScalarTypes[t] {
		// Built-in scalar values are known to be fully and immutably
		// represented by their String() form.
		return v.String(), nil
	}

	return nil, &SnapshotUnsupportedError{ValueType: t.String()}
}

// flagSnapshotRecord is the registration-time portion of a FlagSnapshot.
type flagSnapshotRecord struct {
	defValue string
	value    interface{}
	err      error
}

// captureSnapshot records the registration-time default of flag. It must
// be called with the normalized name of the flag, after the flag has been
// added to the set. It never touches Changed, the actual/formal ordering,
// or any other flag state.
func (f *FlagSet) captureSnapshot(name NormalizedName, flag *Flag) {
	if f.snapshots == nil {
		f.snapshots = make(map[NormalizedName]*flagSnapshotRecord)
	}
	value, err := snapshotValue(flag.Value)
	if serr, ok := err.(*SnapshotUnsupportedError); ok {
		serr.FlagName = flag.Name
	}
	f.snapshots[name] = &flagSnapshotRecord{
		defValue: flag.DefValue,
		value:    value,
		err:      err,
	}
}

// ValueSnapshot returns the explicit default-value snapshot for the named
// flag, together with a query-time report of how the current value differs
// from the registered default. If the flag Value type has no known
// snapshot method, the snapshot is returned with a nil Default and a
// *SnapshotUnsupportedError. Querying a snapshot is read-only: it does not
// modify Changed, the flag ordering, or any other FlagSet state.
func (f *FlagSet) ValueSnapshot(name string) (*FlagSnapshot, error) {
	normalName := f.normalizeFlagName(name)
	flag, ok := f.formal[normalName]
	if !ok {
		return nil, &NotExistError{name: name, messageType: flagNoSuchFlagMessage}
	}

	snap := &FlagSnapshot{
		Name:    flag.Name,
		Current: flag.Value.String(),
		Changed: flag.Changed,
		Source:  f.ChangeSourceOf(name),
	}
	if rec, ok := f.snapshots[normalName]; ok {
		snap.DefValue = rec.defValue
		snap.Default = rec.value
		if rec.err != nil {
			return snap, rec.err
		}
	} else {
		snap.DefValue = flag.DefValue
	}
	return snap, nil
}

// ValueSnapshot returns the default-value snapshot for the named flag on
// the CommandLine flag set.
func ValueSnapshot(name string) (*FlagSnapshot, error) {
	return CommandLine.ValueSnapshot(name)
}

// ChangeSourceOf reports how the named flag was first changed: via
// FlagSet.Parse (ChangeSourceParse), via an explicit FlagSet.Set call
// (ChangeSourceSet), or not at all (ChangeSourceNone). The source is
// recorded directly at the point the value is set; pflag does not re-parse
// arguments to reconstruct it. Querying the source is read-only.
func (f *FlagSet) ChangeSourceOf(name string) ChangeSource {
	normalName := f.normalizeFlagName(name)
	if src, ok := f.changeSources[normalName]; ok {
		return src
	}
	return ChangeSourceNone
}

// ChangeSourceOf reports how the named flag on the CommandLine flag set
// was first changed.
func ChangeSourceOf(name string) ChangeSource {
	return CommandLine.ChangeSourceOf(name)
}
