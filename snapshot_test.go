package pflag

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// customUnregisteredValue is a custom Value with no snapshot support: it
// does not implement SliceValue or ValueSnapshotter and no SnapshotterFunc
// is registered for it.
type customUnregisteredValue struct{ v string }

func (c *customUnregisteredValue) String() string     { return c.v }
func (c *customUnregisteredValue) Set(s string) error { c.v = s; return nil }
func (c *customUnregisteredValue) Type() string       { return "customUnregistered" }

// customRegisteredValue is a custom Value whose snapshot method is
// provided through RegisterValueSnapshotter.
type customRegisteredValue struct{ v string }

func (c *customRegisteredValue) String() string     { return c.v }
func (c *customRegisteredValue) Set(s string) error { c.v = s; return nil }
func (c *customRegisteredValue) Type() string       { return "customRegistered" }

// customSnapshotterValue is a custom Value that implements
// ValueSnapshotter itself.
type customSnapshotterValue struct{ parts []string }

func (c *customSnapshotterValue) String() string { return fmt.Sprint(c.parts) }
func (c *customSnapshotterValue) Set(s string) error {
	c.parts = append(c.parts, s)
	return nil
}
func (c *customSnapshotterValue) Type() string { return "customSnapshotter" }
func (c *customSnapshotterValue) SnapshotValue() (interface{}, error) {
	cp := make([]string, len(c.parts))
	copy(cp, c.parts)
	return cp, nil
}

func TestSnapshotSliceDefaultIndependentOfMutation(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	def := []string{"alpha", "beta"}
	fs.StringSliceVar(new([]string), "tags", def, "usage")

	// Mutate the original default slice after registration. Because the
	// flag value shares the backing array, this also mutates the live
	// flag value; the snapshot must be unaffected.
	def[0] = "MUTATED"

	snap, err := fs.ValueSnapshot("tags")
	if err != nil {
		t.Fatalf("ValueSnapshot returned error: %v", err)
	}
	got, ok := snap.Default.([]string)
	if !ok {
		t.Fatalf("snapshot Default has type %T, want []string", snap.Default)
	}
	want := []string{"alpha", "beta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("snapshot Default = %v, want %v (must be an independent copy)", got, want)
	}
	if snap.DefValue != "[alpha,beta]" {
		t.Errorf("snapshot DefValue = %q, want %q", snap.DefValue, "[alpha,beta]")
	}
	if snap.Changed {
		t.Error("snapshot reports Changed before any set")
	}
	if snap.Source != ChangeSourceNone {
		t.Errorf("snapshot Source = %v, want %v", snap.Source, ChangeSourceNone)
	}
}

func TestSnapshotParseAndSetSources(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	fs.Int("parsed", 0, "usage")
	fs.Int("setted", 0, "usage")
	fs.Int("untouched", 0, "usage")

	if err := fs.Parse([]string{"--parsed", "1"}); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if err := fs.Set("setted", "2"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	cases := []struct {
		name string
		want ChangeSource
	}{
		{"parsed", ChangeSourceParse},
		{"setted", ChangeSourceSet},
		{"untouched", ChangeSourceNone},
	}
	for _, tc := range cases {
		if got := fs.ChangeSourceOf(tc.name); got != tc.want {
			t.Errorf("ChangeSourceOf(%q) = %v, want %v", tc.name, got, tc.want)
		}
		snap, err := fs.ValueSnapshot(tc.name)
		if err != nil {
			t.Fatalf("ValueSnapshot(%q) error: %v", tc.name, err)
		}
		if snap.Source != tc.want {
			t.Errorf("snapshot %q Source = %v, want %v", tc.name, snap.Source, tc.want)
		}
		if wantChanged := tc.want != ChangeSourceNone; snap.Changed != wantChanged {
			t.Errorf("snapshot %q Changed = %v, want %v", tc.name, snap.Changed, wantChanged)
		}
	}

	// The first change wins: a later Parse must not rewrite the source
	// recorded for an earlier explicit Set.
	if err := fs.Parse([]string{"--setted", "3"}); err != nil {
		t.Fatalf("second Parse failed: %v", err)
	}
	if got := fs.ChangeSourceOf("setted"); got != ChangeSourceSet {
		t.Errorf("after re-Parse, ChangeSourceOf(setted) = %v, want %v", got, ChangeSourceSet)
	}
}

func TestSnapshotQueryDoesNotAffectChangedOrOrdering(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	fs.SortFlags = false
	fs.String("charlie", "c", "usage")
	fs.String("alpha", "a", "usage")
	fs.String("bravo", "b", "usage")

	orderBefore := []string{}
	fs.VisitAll(func(fl *Flag) { orderBefore = append(orderBefore, fl.Name) })

	// Query snapshots and sources repeatedly.
	for _, name := range []string{"charlie", "alpha", "bravo"} {
		if _, err := fs.ValueSnapshot(name); err != nil {
			t.Fatalf("ValueSnapshot(%q) error: %v", name, err)
		}
		_ = fs.ChangeSourceOf(name)
	}

	for _, name := range []string{"charlie", "alpha", "bravo"} {
		if fs.Changed(name) {
			t.Errorf("Changed(%q) became true after snapshot queries", name)
		}
	}
	if got := fs.NFlag(); got != 0 {
		t.Errorf("NFlag() = %d after snapshot queries, want 0", got)
	}
	visited := 0
	fs.Visit(func(fl *Flag) { visited++ })
	if visited != 0 {
		t.Errorf("Visit visited %d flags after snapshot queries, want 0", visited)
	}

	orderAfter := []string{}
	fs.VisitAll(func(fl *Flag) { orderAfter = append(orderAfter, fl.Name) })
	if !reflect.DeepEqual(orderBefore, orderAfter) {
		t.Errorf("flag order changed by queries: before %v, after %v", orderBefore, orderAfter)
	}
	wantOrder := []string{"charlie", "alpha", "bravo"}
	if !reflect.DeepEqual(orderAfter, wantOrder) {
		t.Errorf("flag order = %v, want insertion order %v", orderAfter, wantOrder)
	}
}

func TestSnapshotCustomValueRejected(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	cv := &customUnregisteredValue{v: "dflt"}
	fs.Var(cv, "custom", "usage")

	snap, err := fs.ValueSnapshot("custom")
	if err == nil {
		t.Fatal("ValueSnapshot succeeded for custom value without a registered snapshotter")
	}
	var unsup *SnapshotUnsupportedError
	if !errors.As(err, &unsup) {
		t.Fatalf("error has type %T, want *SnapshotUnsupportedError", err)
	}
	if unsup.FlagName != "custom" {
		t.Errorf("SnapshotUnsupportedError.FlagName = %q, want %q", unsup.FlagName, "custom")
	}
	if snap == nil {
		t.Fatal("snapshot is nil; want partial snapshot with nil Default")
	}
	if snap.Default != nil {
		t.Errorf("snapshot Default = %v, want nil for rejected custom value", snap.Default)
	}
	if fs.Changed("custom") {
		t.Error("Changed(custom) became true after rejected snapshot query")
	}
}

func TestSnapshotCustomValueRegistered(t *testing.T) {
	RegisterValueSnapshotter((*customRegisteredValue)(nil), func(v Value) (interface{}, error) {
		return v.(*customRegisteredValue).v, nil
	})

	fs := NewFlagSet("test", ContinueOnError)
	cv := &customRegisteredValue{v: "dflt"}
	fs.Var(cv, "custom", "usage")
	cv.v = "changed-after-registration"

	snap, err := fs.ValueSnapshot("custom")
	if err != nil {
		t.Fatalf("ValueSnapshot returned error: %v", err)
	}
	if snap.Default != "dflt" {
		t.Errorf("snapshot Default = %v, want %q", snap.Default, "dflt")
	}
	if snap.Current != "changed-after-registration" {
		t.Errorf("snapshot Current = %q, want %q", snap.Current, "changed-after-registration")
	}
}

func TestSnapshotCustomValueSnapshotterInterface(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	cv := &customSnapshotterValue{parts: []string{"x", "y"}}
	fs.Var(cv, "custom", "usage")
	cv.parts[0] = "MUTATED"

	snap, err := fs.ValueSnapshot("custom")
	if err != nil {
		t.Fatalf("ValueSnapshot returned error: %v", err)
	}
	want := []string{"x", "y"}
	if !reflect.DeepEqual(snap.Default, want) {
		t.Errorf("snapshot Default = %v, want %v", snap.Default, want)
	}
}

func TestSnapshotBoolNoOptSemanticsPreserved(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	verbose := fs.Bool("verbose", false, "usage")
	count := fs.CountP("count", "c", "usage")

	if err := fs.Parse([]string{"--verbose", "-cc"}); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !*verbose {
		t.Error("bool no-opt flag was not set to true by bare --verbose")
	}
	if *count != 2 {
		t.Errorf("count = %d, want 2 (no-opt +1 semantics)", *count)
	}
	// NoOptDefVal semantics must be untouched by snapshotting.
	if got := fs.Lookup("verbose").NoOptDefVal; got != "true" {
		t.Errorf("NoOptDefVal(verbose) = %q, want %q", got, "true")
	}
	if got := fs.Lookup("count").NoOptDefVal; got != "+1" {
		t.Errorf("NoOptDefVal(count) = %q, want %q", got, "+1")
	}
	if got := fs.ChangeSourceOf("verbose"); got != ChangeSourceParse {
		t.Errorf("ChangeSourceOf(verbose) = %v, want %v", got, ChangeSourceParse)
	}
	if got := fs.ChangeSourceOf("count"); got != ChangeSourceParse {
		t.Errorf("ChangeSourceOf(count) = %v, want %v", got, ChangeSourceParse)
	}

	snap, err := fs.ValueSnapshot("verbose")
	if err != nil {
		t.Fatalf("ValueSnapshot(verbose) error: %v", err)
	}
	if snap.Default != "false" || snap.DefValue != "false" {
		t.Errorf("bool snapshot Default/DefValue = %v/%q, want false/false", snap.Default, snap.DefValue)
	}
	if snap.Current != "true" {
		t.Errorf("bool snapshot Current = %q, want %q", snap.Current, "true")
	}
}

func TestSnapshotScalarDefaults(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	fs.Int("number", 42, "usage")
	fs.String("word", "hello", "usage")

	snap, err := fs.ValueSnapshot("number")
	if err != nil {
		t.Fatalf("ValueSnapshot(number) error: %v", err)
	}
	if snap.Default != "42" {
		t.Errorf("int snapshot Default = %v, want %q", snap.Default, "42")
	}

	snap, err = fs.ValueSnapshot("word")
	if err != nil {
		t.Fatalf("ValueSnapshot(word) error: %v", err)
	}
	if snap.Default != "hello" {
		t.Errorf("string snapshot Default = %v, want %q", snap.Default, "hello")
	}
}

func TestSnapshotUnknownFlag(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	if _, err := fs.ValueSnapshot("nope"); err == nil {
		t.Error("ValueSnapshot(nope) succeeded, want NotExistError")
	}
	if got := fs.ChangeSourceOf("nope"); got != ChangeSourceNone {
		t.Errorf("ChangeSourceOf(nope) = %v, want %v", got, ChangeSourceNone)
	}
}

func ExampleFlagSet_ValueSnapshot() {
	fs := NewFlagSet("example", ContinueOnError)
	defaults := []string{"alpha", "beta"}
	fs.StringSlice("tags", defaults, "tags to apply")

	// Mutating the original default slice after registration does not
	// affect the snapshot.
	defaults[0] = "MUTATED"

	_ = fs.Parse([]string{"--tags", "gamma"})

	snap, err := fs.ValueSnapshot("tags")
	if err != nil {
		fmt.Println("snapshot error:", err)
		return
	}
	fmt.Println("default:", snap.Default)
	fmt.Println("current:", snap.Current)
	fmt.Println("changed:", snap.Changed)
	fmt.Println("source:", snap.Source)
	// Output:
	// default: [alpha beta]
	// current: [gamma]
	// changed: true
	// source: parse
}

func ExampleRegisterValueSnapshotter() {
	RegisterValueSnapshotter((*customRegisteredValue)(nil), func(v Value) (interface{}, error) {
		// Return an independent copy of the current value.
		return v.(*customRegisteredValue).v, nil
	})

	fs := NewFlagSet("example", ContinueOnError)
	fs.Var(&customRegisteredValue{v: "dflt"}, "custom", "usage")
	_ = fs.Set("custom", "new")

	snap, err := fs.ValueSnapshot("custom")
	if err != nil {
		fmt.Println("snapshot error:", err)
		return
	}
	fmt.Println("default:", snap.Default)
	fmt.Println("current:", snap.Current)
	fmt.Println("source:", snap.Source)
	// Output:
	// default: dflt
	// current: new
	// source: set
}
