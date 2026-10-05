// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pflag

import (
	"fmt"
	"net"
	"time"
)

// ChangeSource identifies the way in which a flag s current value came
// to differ from the default value it was registered with.
type ChangeSource int

const (
	// SourceDefault means the flag still holds its registered default value.
	SourceDefault ChangeSource = iota
	// SourceParse means the value was changed by an explicit call to Parse.
	SourceParse
	// SourceSet means the value was changed by a direct call to Set.
	SourceSet
)

// String returns a human-readable name for the change source.
func (s ChangeSource) String() string {
	switch s {
	case SourceDefault:
		return "default"
	case SourceParse:
		return "parse"
	case SourceSet:
		return "set"
	default:
		return fmt.Sprintf("ChangeSource(%d)", int(s))
	}
}

// ValueSnapshot is implemented by Value types that can produce an
// independent copy of themselves. The returned Value must not share any
// mutable state (slices, maps, pointers, ...) with the receiver: later
// mutations of the receiver must not affect the returned copy, and vice
// versa.
//
// pflag records such copies as default snapshots when flags are
// registered, so that the registered default can be compared against the
// current value at any time. Snapshotting never guesses a value s shape
// from its String representation: a Value either implements this
// interface or is one of the built-in types pflag knows how to copy.
type ValueSnapshot interface {
	SnapshotValue() Value
}

// DefaultSnapshot is an immutable record of a flag s value at the time
// the flag was registered. The snapshot is a deep copy, so later
// mutations of the value the caller passed in do not affect it.
type DefaultSnapshot struct {
	name  string
	flag  *Flag
	value Value
	err   error
}

// Name returns the name of the snapshotted flag.
func (s *DefaultSnapshot) Name() string { return s.name }

// Value returns the value captured at registration time. It returns an
// error if the flag s value type has no snapshot strategy; in that case
// the snapshot is rejected rather than guessed.
func (s *DefaultSnapshot) Value() (Value, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.value, nil
}

// Current returns the flag s current value. It never returns an error;
// if the flag has been removed it returns nil.
func (s *DefaultSnapshot) Current() Value {
	if s.flag == nil {
		return nil
	}
	return s.flag.Value
}

// Changed reports whether the flag s current value differs from its
// registered default. Querying the snapshot does not modify the flag s
// Changed state.
func (s *DefaultSnapshot) Changed() bool {
	if s.flag == nil {
		return false
	}
	return s.flag.Changed
}

// Source reports how the flag s current value came to differ from the
// registered default. It returns SourceDefault if the flag still holds
// its default, SourceParse if it was changed by an explicit Parse call,
// and SourceSet if it was changed by a direct Set call.
func (s *DefaultSnapshot) Source() ChangeSource {
	if s.flag == nil {
		return SourceDefault
	}
	return s.flag.changeSource
}

// DefaultSnapshot returns the default snapshot for the named flag. The
// snapshot is a deep copy of the value at registration time. It returns
// an error if the flag does not exist or if its value type has no
// snapshot strategy.
func (f *FlagSet) DefaultSnapshot(name string) (*DefaultSnapshot, error) {
	flag := f.Lookup(name)
	if flag == nil {
		return nil, fmt.Errorf("flag %q not found", name)
	}
	return &DefaultSnapshot{
		name:  flag.Name,
		flag:  flag,
		value: flag.defaultSnapshot,
		err:   flag.snapshotErr,
	}, nil
}

// DefaultSnapshots returns the default snapshots for all flags in the
// set, in registration (primordial) order. The returned slice does not
// sort or otherwise reorder the flags.
func (f *FlagSet) DefaultSnapshots() []*DefaultSnapshot {
	result := make([]*DefaultSnapshot, 0, len(f.orderedFormal))
	for _, flag := range f.orderedFormal {
		result = append(result, &DefaultSnapshot{
			name:  flag.Name,
			flag:  flag,
			value: flag.defaultSnapshot,
			err:   flag.snapshotErr,
		})
	}
	return result
}

// SnapshotChanged reports whether the current value of the named flag
// differs from its registered default snapshot. Querying does not modify
// the flag s Changed state. It returns an error if the flag does not
// exist or if its value type has no snapshot strategy.
func (f *FlagSet) SnapshotChanged(name string) (bool, error) {
	snap, err := f.DefaultSnapshot(name)
	if err != nil {
		return false, err
	}
	if snap.err != nil {
		return false, snap.err
	}
	return snap.Changed(), nil
}

// ChangeSource reports how the current value of the named flag came to
// differ from its registered default. It returns an error if the flag
// does not exist or if its value type has no snapshot strategy.
func (f *FlagSet) ChangeSource(name string) (ChangeSource, error) {
	snap, err := f.DefaultSnapshot(name)
	if err != nil {
		return SourceDefault, err
	}
	if snap.err != nil {
		return SourceDefault, snap.err
	}
	return snap.Source(), nil
}

// snapshotValue returns an independent copy of v for use as a default
// snapshot. It returns false if v s type has no snapshot strategy;
// callers must reject such values rather than guessing their shape from
// String().
func snapshotValue(v Value) (Value, bool) {
	if v == nil {
		return nil, false
	}
	if s, ok := v.(ValueSnapshot); ok {
		return s.SnapshotValue(), true
	}
	switch x := v.(type) {
	case *boolValue:
		c := *x
		return &c, true
	case *stringValue:
		c := *x
		return &c, true
	case *intValue:
		c := *x
		return &c, true
	case *int8Value:
		c := *x
		return &c, true
	case *int16Value:
		c := *x
		return &c, true
	case *int32Value:
		c := *x
		return &c, true
	case *int64Value:
		c := *x
		return &c, true
	case *uintValue:
		c := *x
		return &c, true
	case *uint8Value:
		c := *x
		return &c, true
	case *uint16Value:
		c := *x
		return &c, true
	case *uint32Value:
		c := *x
		return &c, true
	case *uint64Value:
		c := *x
		return &c, true
	case *float32Value:
		c := *x
		return &c, true
	case *float64Value:
		c := *x
		return &c, true
	case *durationValue:
		c := *x
		return &c, true
	case *countValue:
		c := *x
		return &c, true
	case *ipValue:
		out := make(net.IP, len(*x))
		copy(out, *x)
		return (*ipValue)(&out), true
	case *ipMaskValue:
		out := make(net.IPMask, len(*x))
		copy(out, *x)
		return (*ipMaskValue)(&out), true
	case *ipNetValue:
		return (*ipNetValue)(cloneIPNet((*net.IPNet)(x))), true
	case *bytesHexValue:
		out := make([]byte, len(*x))
		copy(out, *x)
		return (*bytesHexValue)(&out), true
	case *bytesBase64Value:
		out := make([]byte, len(*x))
		copy(out, *x)
		return (*bytesBase64Value)(&out), true
	case *stringSliceValue:
		return &stringSliceValue{value: cloneStringSlicePtr(x.value)}, true
	case *stringArrayValue:
		return &stringArrayValue{value: cloneStringSlicePtr(x.value)}, true
	case *boolSliceValue:
		return &boolSliceValue{value: cloneBoolSlicePtr(x.value)}, true
	case *durationSliceValue:
		return &durationSliceValue{value: cloneDurationSlicePtr(x.value)}, true
	case *float32SliceValue:
		return &float32SliceValue{value: cloneFloat32SlicePtr(x.value)}, true
	case *float64SliceValue:
		return &float64SliceValue{value: cloneFloat64SlicePtr(x.value)}, true
	case *int32SliceValue:
		return &int32SliceValue{value: cloneInt32SlicePtr(x.value)}, true
	case *int64SliceValue:
		return &int64SliceValue{value: cloneInt64SlicePtr(x.value)}, true
	case *intSliceValue:
		return &intSliceValue{value: cloneIntSlicePtr(x.value)}, true
	case *uintSliceValue:
		return &uintSliceValue{value: cloneUintSlicePtr(x.value)}, true
	case *ipSliceValue:
		return &ipSliceValue{value: cloneIPSlicePtr(x.value)}, true
	case *ipNetSliceValue:
		return &ipNetSliceValue{value: cloneIPNetSlicePtr(x.value)}, true
	case *stringToStringValue:
		return &stringToStringValue{value: cloneStringMapPtr(x.value)}, true
	case *stringToIntValue:
		return &stringToIntValue{value: cloneIntMapPtr(x.value)}, true
	case *stringToInt64Value:
		return &stringToInt64Value{value: cloneInt64MapPtr(x.value)}, true
	case *timeValue:
		return cloneTimeValue(x), true
	}
	return nil, false
}

func cloneStringSlicePtr(p *[]string) *[]string {
	if p == nil {
		return nil
	}
	out := make([]string, len(*p))
	copy(out, *p)
	return &out
}

func cloneBoolSlicePtr(p *[]bool) *[]bool {
	if p == nil {
		return nil
	}
	out := make([]bool, len(*p))
	copy(out, *p)
	return &out
}

func cloneDurationSlicePtr(p *[]time.Duration) *[]time.Duration {
	if p == nil {
		return nil
	}
	out := make([]time.Duration, len(*p))
	copy(out, *p)
	return &out
}

func cloneFloat32SlicePtr(p *[]float32) *[]float32 {
	if p == nil {
		return nil
	}
	out := make([]float32, len(*p))
	copy(out, *p)
	return &out
}

func cloneFloat64SlicePtr(p *[]float64) *[]float64 {
	if p == nil {
		return nil
	}
	out := make([]float64, len(*p))
	copy(out, *p)
	return &out
}

func cloneInt32SlicePtr(p *[]int32) *[]int32 {
	if p == nil {
		return nil
	}
	out := make([]int32, len(*p))
	copy(out, *p)
	return &out
}

func cloneInt64SlicePtr(p *[]int64) *[]int64 {
	if p == nil {
		return nil
	}
	out := make([]int64, len(*p))
	copy(out, *p)
	return &out
}

func cloneIntSlicePtr(p *[]int) *[]int {
	if p == nil {
		return nil
	}
	out := make([]int, len(*p))
	copy(out, *p)
	return &out
}

func cloneUintSlicePtr(p *[]uint) *[]uint {
	if p == nil {
		return nil
	}
	out := make([]uint, len(*p))
	copy(out, *p)
	return &out
}

func cloneIPSlicePtr(p *[]net.IP) *[]net.IP {
	if p == nil {
		return nil
	}
	out := make([]net.IP, len(*p))
	for i, ip := range *p {
		if ip == nil {
			continue
		}
		cp := make(net.IP, len(ip))
		copy(cp, ip)
		out[i] = cp
	}
	return &out
}

func cloneIPNetSlicePtr(p *[]net.IPNet) *[]net.IPNet {
	if p == nil {
		return nil
	}
	out := make([]net.IPNet, len(*p))
	for i := range *p {
		out[i] = *cloneIPNet(&(*p)[i])
	}
	return &out
}

func cloneIPNet(n *net.IPNet) *net.IPNet {
	if n == nil {
		return nil
	}
	var ip net.IP
	if n.IP != nil {
		ip = make(net.IP, len(n.IP))
		copy(ip, n.IP)
	}
	var mask net.IPMask
	if n.Mask != nil {
		mask = make(net.IPMask, len(n.Mask))
		copy(mask, n.Mask)
	}
	return &net.IPNet{IP: ip, Mask: mask}
}

func cloneStringMapPtr(p *map[string]string) *map[string]string {
	if p == nil {
		return nil
	}
	out := make(map[string]string, len(*p))
	for k, v := range *p {
		out[k] = v
	}
	return &out
}

func cloneIntMapPtr(p *map[string]int) *map[string]int {
	if p == nil {
		return nil
	}
	out := make(map[string]int, len(*p))
	for k, v := range *p {
		out[k] = v
	}
	return &out
}

func cloneInt64MapPtr(p *map[string]int64) *map[string]int64 {
	if p == nil {
		return nil
	}
	out := make(map[string]int64, len(*p))
	for k, v := range *p {
		out[k] = v
	}
	return &out
}

func cloneTimeValue(x *timeValue) *timeValue {
	var t time.Time
	if x.Time != nil {
		t = *x.Time
	}
	formats := make([]string, len(x.formats))
	copy(formats, x.formats)
	return &timeValue{Time: &t, formats: formats}
}
