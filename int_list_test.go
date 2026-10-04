package pflag

import (
	"reflect"
	"testing"
)

func setUpILFlagSet(ilp *[]int) *FlagSet {
	f := NewFlagSet("test", ContinueOnError)
	f.IntListVar(ilp, "il", []int{}, "list operations")
	return f
}

func setUpILFlagSetWithDefault(ilp *[]int) *FlagSet {
	f := NewFlagSet("test", ContinueOnError)
	f.IntListVar(ilp, "il", []int{1, 2}, "list operations")
	return f
}

func TestIntListEmpty(t *testing.T) {
	var il []int
	f := setUpILFlagSet(&il)
	err := f.Parse([]string{})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if len(il) != 0 {
		t.Fatalf("got il %v with len=%d but expected length=0", il, len(il))
	}
}

func TestIntListDefault(t *testing.T) {
	var il []int
	f := setUpILFlagSetWithDefault(&il)

	err := f.Parse([]string{})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if !reflect.DeepEqual(il, []int{1, 2}) {
		t.Fatalf("expected default [1,2] but got %v", il)
	}

	got, err := f.GetIntList("il")
	if err != nil {
		t.Fatal("got an error from GetIntList():", err)
	}
	if !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("expected GetIntList [1,2] but got %v", got)
	}
}

func TestIntListReplace(t *testing.T) {
	var il []int
	f := setUpILFlagSetWithDefault(&il)

	err := f.Parse([]string{"--il==3,4,5"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if !reflect.DeepEqual(il, []int{3, 4, 5}) {
		t.Fatalf("expected [3,4,5] but got %v", il)
	}
}

func TestIntListAppend(t *testing.T) {
	var il []int
	f := setUpILFlagSetWithDefault(&il)

	err := f.Parse([]string{"--il=+3", "--il=+4"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if !reflect.DeepEqual(il, []int{1, 2, 3, 4}) {
		t.Fatalf("expected [1,2,3,4] but got %v", il)
	}
}

func TestIntListDelete(t *testing.T) {
	var il []int
	f := NewFlagSet("test", ContinueOnError)
	f.IntListVar(&il, "il", []int{1, 2, 1, 3, 1}, "list operations")

	err := f.Parse([]string{"--il=-1"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if !reflect.DeepEqual(il, []int{2, 3}) {
		t.Fatalf("expected [2,3] (all 1s deleted) but got %v", il)
	}
}

// TestIntListAcceptance is the headline scenario: baseline [1,2], then +3,
// -1, +2 yields [2,3,2], and a subsequent +bad errors while preserving the
// baseline (the value before that Set call).
func TestIntListAcceptance(t *testing.T) {
	var il []int
	f := setUpILFlagSetWithDefault(&il)

	err := f.Parse([]string{"--il=+3", "--il=-1", "--il=+2"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	expected := []int{2, 3, 2}
	if !reflect.DeepEqual(il, expected) {
		t.Fatalf("expected %v but got %v", expected, il)
	}
	if !f.Changed("il") {
		t.Fatal("expected Changed to be true after successful operations")
	}

	// A later +bad errors and preserves the baseline (the value before this
	// Set call, which is the result of the previous parse).
	err = f.Parse([]string{"--il=+bad"})
	if err == nil {
		t.Fatal("expected error for +bad; got nil")
	}
	if !reflect.DeepEqual(il, expected) {
		t.Fatalf("expected baseline %v to be preserved after error; got %v", expected, il)
	}
}

// TestIntListAtomicInSingleParse checks that when several occurrences appear
// in one Parse and the last one is invalid, the successful ones are kept and
// the value is the baseline before the failed Set (per-Set atomicity).
func TestIntListAtomicInSingleParse(t *testing.T) {
	var il []int
	f := setUpILFlagSetWithDefault(&il)

	err := f.Parse([]string{"--il=+3", "--il=-1", "--il=+2", "--il=+bad"})
	if err == nil {
		t.Fatal("expected error; got nil")
	}
	expected := []int{2, 3, 2}
	if !reflect.DeepEqual(il, expected) {
		t.Fatalf("expected %v (baseline before failed Set) but got %v", expected, il)
	}
}

// TestIntListNegative checks the distinguishable syntax for negative
// integers: +(-1) appends -1, -(-1) deletes every -1, and -1 deletes every 1
// (it is not the negative integer).
func TestIntListNegative(t *testing.T) {
	var il []int
	f := NewFlagSet("test", ContinueOnError)
	f.IntListVar(&il, "il", []int{1, -2, 3}, "list operations")

	err := f.Parse([]string{"--il=+(-1)"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if !reflect.DeepEqual(il, []int{1, -2, 3, -1}) {
		t.Fatalf("expected [1,-2,3,-1] but got %v", il)
	}

	err = f.Parse([]string{"--il=-(-2)"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if !reflect.DeepEqual(il, []int{1, 3, -1}) {
		t.Fatalf("expected [1,3,-1] (all -2s deleted) but got %v", il)
	}

	// -1 deletes every 1; it is not the negative integer.
	err = f.Parse([]string{"--il=-1"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if !reflect.DeepEqual(il, []int{3, -1}) {
		t.Fatalf("expected [3,-1] (all 1s deleted) but got %v", il)
	}
}

// TestIntListBareNegativeRejected checks that a bare negative integer in the
// append/delete operand is rejected (it must be parenthesized).
func TestIntListBareNegativeRejected(t *testing.T) {
	var il []int
	f := setUpILFlagSetWithDefault(&il)

	for _, arg := range []string{"--il=+-1", "--il=--1"} {
		err := f.Parse([]string{arg})
		if err == nil {
			t.Fatalf("expected error for %s; got nil", arg)
		}
		if !reflect.DeepEqual(il, []int{1, 2}) {
			t.Fatalf("expected baseline [1,2] to be preserved after %s; got %v", arg, il)
		}
	}
}

// TestIntListEmptyReplacement checks that an empty operand clears the list.
func TestIntListEmptyReplacement(t *testing.T) {
	var il []int
	f := setUpILFlagSetWithDefault(&il)

	err := f.Parse([]string{"--il=="})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if len(il) != 0 {
		t.Fatalf("expected empty list but got %v", il)
	}
}

// TestIntListChanged checks that Changed is only set after a successful
// operation.
func TestIntListChanged(t *testing.T) {
	var il []int
	f := setUpILFlagSetWithDefault(&il)

	if f.Changed("il") {
		t.Fatal("expected Changed to be false before parse")
	}

	err := f.Parse([]string{"--il=+bad"})
	if err == nil {
		t.Fatal("expected error; got nil")
	}
	if f.Changed("il") {
		t.Fatal("expected Changed to remain false after failed parse")
	}

	err = f.Parse([]string{"--il=+3"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if !f.Changed("il") {
		t.Fatal("expected Changed to be true after successful parse")
	}
}

// TestIntListRepeatedParsing documents the baseline semantics across Parse
// calls: each Parse starts from the value at the end of the previous Parse
// (the baseline rolls forward).
func TestIntListRepeatedParsing(t *testing.T) {
	var il []int
	f := setUpILFlagSetWithDefault(&il)

	err := f.Parse([]string{"--il=+3"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if !reflect.DeepEqual(il, []int{1, 2, 3}) {
		t.Fatalf("expected [1,2,3] but got %v", il)
	}

	// The second Parse starts from the current value [1,2,3].
	err = f.Parse([]string{"--il=+4"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if !reflect.DeepEqual(il, []int{1, 2, 3, 4}) {
		t.Fatalf("expected [1,2,3,4] but got %v", il)
	}
}

// TestIntListUnknownOperation checks that an unknown operator errors and
// preserves the baseline.
func TestIntListUnknownOperation(t *testing.T) {
	var il []int
	f := setUpILFlagSetWithDefault(&il)

	err := f.Parse([]string{"--il=*3"})
	if err == nil {
		t.Fatal("expected error for unknown operation; got nil")
	}
	if !reflect.DeepEqual(il, []int{1, 2}) {
		t.Fatalf("expected baseline [1,2] to be preserved; got %v", il)
	}
}

// TestIntListTypeAndString checks the Type and String methods.
func TestIntListTypeAndString(t *testing.T) {
	var il []int
	f := setUpILFlagSetWithDefault(&il)

	flag := f.Lookup("il")
	if flag.Value.Type() != "intList" {
		t.Fatalf("expected type intList but got %q", flag.Value.Type())
	}
	if flag.Value.String() != "[1,2]" {
		t.Fatalf("expected String() \"[1,2]\" but got %q", flag.Value.String())
	}
}

// TestIntListConstructor checks the IntList constructor returning *[]int.
func TestIntListConstructor(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	p := f.IntList("il", []int{1, 2}, "list operations")
	if p == nil {
		t.Fatal("expected non-nil pointer")
	}
	err := f.Parse([]string{"--il=+3"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if !reflect.DeepEqual(*p, []int{1, 2, 3}) {
		t.Fatalf("expected [1,2,3] but got %v", *p)
	}
}

// TestIntListPreservesOrder checks that element order is preserved through
// append and delete operations.
func TestIntListPreservesOrder(t *testing.T) {
	var il []int
	f := NewFlagSet("test", ContinueOnError)
	f.IntListVar(&il, "il", []int{5, 1, 4, 2, 3}, "list operations")

	err := f.Parse([]string{"--il=-1", "--il=+9"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if !reflect.DeepEqual(il, []int{5, 4, 2, 3, 9}) {
		t.Fatalf("expected [5,4,2,3,9] but got %v", il)
	}
}

// TestIntListShorthand checks the shorthand constructor.
func TestIntListShorthand(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	var il []int
	f.IntListVarP(&il, "il", "i", []int{1, 2}, "list operations")

	err := f.Parse([]string{"-i+3"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	if !reflect.DeepEqual(il, []int{1, 2, 3}) {
		t.Fatalf("expected [1,2,3] but got %v", il)
	}
}
