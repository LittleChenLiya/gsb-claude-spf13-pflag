package pflag

import (
	"reflect"
	"strings"
	"testing"
)

func setUpILFlagSet(p *[]int, def []int) *FlagSet {
	f := NewFlagSet("test", ContinueOnError)
	f.IntListVar(p, "il", def, "Registerable int list")
	return f
}

func TestIntListAcceptance(t *testing.T) {
	var il []int
	f := setUpILFlagSet(&il, []int{1, 2})

	// Baseline [1,2], then +3, -1, +2 applied in order -> [2,3,2].
	err := f.Parse([]string{"--il=+3", "--il=-1", "--il=+2"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if want := []int{2, 3, 2}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected %v, got %v", want, il)
	}
	if !f.Changed("il") {
		t.Fatal("expected the flag to report Changed=true after a successful parse")
	}

	// A follow-up parse with an invalid append must error and keep the
	// current baseline ([2,3,2]) untouched.
	err = f.Parse([]string{"--il=+bad"})
	if err == nil {
		t.Fatal("expected an error parsing +bad, got nil")
	}
	if want := []int{2, 3, 2}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected the baseline %v to be retained, got %v", want, il)
	}
	// The previous successful parse still counts as a change.
	if !f.Changed("il") {
		t.Fatal("expected Changed to remain true after a failed later parse")
	}
}

func TestIntListAtomicRollbackRestoresParseBaseline(t *testing.T) {
	var il []int
	f := setUpILFlagSet(&il, []int{1, 2})

	// Operations within one Parse are atomic: the two valid occurrences
	// are discarded when the last one fails and the value goes back to the
	// baseline this parse started from.
	err := f.Parse([]string{"--il=+3", "--il=-1", "--il=+2", "--il=+bad"})
	if err == nil {
		t.Fatal("expected an error parsing +bad, got nil")
	}
	if want := []int{1, 2}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected rollback to baseline %v, got %v", want, il)
	}
	if f.Changed("il") {
		t.Fatal("expected Changed to stay false when the whole parse failed")
	}
}

func TestIntListReplace(t *testing.T) {
	var il []int
	f := setUpILFlagSet(&il, []int{1, 2})

	if err := f.Parse([]string{"--il=9,8,7"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{9, 8, 7}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected %v, got %v", want, il)
	}

	// A bare value without an explicit operator also replaces.
	if err := f.Parse([]string{"--il=4"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{4}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected %v, got %v", want, il)
	}

	// An explicit leading '=' is a replacement too.
	if err := f.Parse([]string{"--il==6,7"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{6, 7}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected %v, got %v", want, il)
	}
}

func TestIntListEmptyReplacement(t *testing.T) {
	var il []int
	f := setUpILFlagSet(&il, []int{1, 2})

	for _, arg := range []string{"--il=", "--il=="} {
		if err := f.Parse([]string{arg}); err != nil {
			t.Fatalf("unexpected error for %q: %v", arg, err)
		}
		if len(il) != 0 {
			t.Fatalf("expected empty list for %q, got %v", arg, il)
		}
		if !f.Changed("il") {
			t.Fatalf("expected Changed=true for %q", arg)
		}
	}
}

func TestIntListRemoveAllMatches(t *testing.T) {
	var il []int
	f := setUpILFlagSet(&il, []int{1, 2, 1, 3, 1})

	if err := f.Parse([]string{"--il=-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{2, 3}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected %v, got %v", want, il)
	}
}

func TestIntListNegativeIntegersAreUnambiguous(t *testing.T) {
	var il []int
	f := setUpILFlagSet(&il, []int{})

	// "+-3" appends -3; "--3" removes every -3; "-3" removes positive 3.
	if err := f.Parse([]string{"--il=+-3", "--il=5", "--il=--3"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{5}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected --3 to remove -3, got %v", il)
	}

	// Append -3 again and prove that a bare "-3" targets positive 3, so the
	// negative element survives and 3 (if present) is removed.
	if err := f.Parse([]string{"--il=+-3", "--il=+3", "--il=-3"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{5, -3}; !reflect.DeepEqual(il, want) {
		t.Fatalf("removing 3 should not touch -3; expected %v, got %v", want, il)
	}
}

func TestIntListDefaultBaselineWhenUnset(t *testing.T) {
	var il []int
	f := setUpILFlagSet(&il, []int{1, 2})

	if err := f.Parse(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{1, 2}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected default baseline %v, got %v", want, il)
	}
	if f.Changed("il") {
		t.Fatal("expected Changed=false when the flag was never provided")
	}
}

func TestIntListRepeatedParsesStartFromCommittedBaseline(t *testing.T) {
	var il []int
	f := setUpILFlagSet(&il, []int{1})

	if err := f.Parse([]string{"--il=+2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{1, 2}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected %v, got %v", want, il)
	}

	// A failed parse rolls back to the value committed by the previous
	// successful parse, not to the registered default.
	if err := f.Parse([]string{"--il=+9", "--il=+bad"}); err == nil {
		t.Fatal("expected an error, got nil")
	}
	if want := []int{1, 2}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected committed baseline %v, got %v", want, il)
	}

	// A subsequent successful parse still starts from [1,2].
	if err := f.Parse([]string{"--il=-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{2}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected %v, got %v", want, il)
	}
}

func TestIntListConversionError(t *testing.T) {
	var il []int
	f := setUpILFlagSet(&il, []int{1, 2})

	err := f.Parse([]string{"--il=bad"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	// Keep the conversion-error habit of the other pflag types.
	if !strings.Contains(err.Error(), "invalid argument") || !strings.Contains(err.Error(), `"bad"`) {
		t.Fatalf("expected an InvalidValueError style message, got %v", err)
	}
	if want := []int{1, 2}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected baseline %v retained, got %v", want, il)
	}
}

func TestIntListOperatorWithoutValue(t *testing.T) {
	var il []int
	f := setUpILFlagSet(&il, []int{1})

	if err := f.Parse([]string{"--il=+"}); err == nil {
		t.Fatal("expected an error for an append without a value")
	}
	if want := []int{1}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected baseline %v retained, got %v", want, il)
	}
}

func TestIntListStringAndType(t *testing.T) {
	var il []int
	f := setUpILFlagSet(&il, []int{1, 2})
	flag := f.Lookup("il")

	if got := flag.Value.Type(); got != "intList" {
		t.Fatalf("expected type intList, got %q", got)
	}
	if got := flag.Value.String(); got != "[1,2]" {
		t.Fatalf("expected [1,2], got %q", got)
	}

	if err := f.Parse([]string{"--il=+3"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := flag.Value.String(); got != "[1,2,3]" {
		t.Fatalf("expected [1,2,3], got %q", got)
	}
}

func TestGetIntList(t *testing.T) {
	var il []int
	f := setUpILFlagSet(&il, []int{1, 2})

	if err := f.Parse([]string{"--il=-2", "--il=+4"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := f.GetIntList("il")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{1, 4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, il)
	}
}

func TestIntListSetOutsideParse(t *testing.T) {
	var il []int
	f := setUpILFlagSet(&il, []int{1, 2})

	// Direct Set applies immediately with no parse transaction around it.
	if err := f.Set("il", "+3"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{1, 2, 3}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected %v, got %v", want, il)
	}
	if err := f.Set("il", "-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{2, 3}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected %v, got %v", want, il)
	}
}

func TestIntListRollbackDoesNotTouchOtherFlags(t *testing.T) {
	var il []int
	var other string
	f := NewFlagSet("test", ContinueOnError)
	f.IntListVar(&il, "il", []int{1, 2}, "Registerable int list")
	f.StringVar(&other, "other", "default", "plain string flag")

	// A regular flag parsed before the intList failure keeps its value and
	// its Changed state; only the intList flag is rolled back.
	err := f.Parse([]string{"--other=set", "--il=+3", "--il=+bad"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if other != "set" {
		t.Fatalf("expected other flag to keep %q, got %q", "set", other)
	}
	if !f.Changed("other") {
		t.Fatal("expected the other flag to remain Changed")
	}
	if want := []int{1, 2}; !reflect.DeepEqual(il, want) {
		t.Fatalf("expected intList baseline %v, got %v", want, il)
	}
	if f.Changed("il") {
		t.Fatal("expected intList to be un-Changed after rollback")
	}
}

func TestIntListHelpDoesNotRollback(t *testing.T) {
	var il []int
	f := NewFlagSet("test", ContinueOnError)
	f.IntListVar(&il, "il", []int{1, 2}, "Registerable int list")

	err := f.Parse([]string{"--il=+3", "--help"})
	if err != ErrHelp {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
	if want := []int{1, 2, 3}; !reflect.DeepEqual(il, want) {
		t.Fatalf("help request should keep prior values, expected %v, got %v", want, il)
	}
	if !f.Changed("il") {
		t.Fatal("expected Changed=true for the flags set before --help")
	}
}

func TestIntListMultipleTransactionalFlagsRollBack(t *testing.T) {
	var a, b []int
	f := NewFlagSet("test", ContinueOnError)
	f.IntListVar(&a, "a", []int{1}, "first list")
	f.IntListVar(&b, "b", []int{9}, "second list")

	err := f.Parse([]string{"--a=+2", "--b=+8", "--a=+bad"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if want := []int{1}; !reflect.DeepEqual(a, want) {
		t.Fatalf("expected a baseline %v, got %v", want, a)
	}
	if want := []int{9}; !reflect.DeepEqual(b, want) {
		t.Fatalf("expected b baseline %v, got %v", want, b)
	}
	if f.Changed("a") || f.Changed("b") {
		t.Fatal("expected neither list flag to report Changed")
	}

	// Visit must not surface the rolled-back flags.
	seen := map[string]bool{}
	f.Visit(func(fl *Flag) { seen[fl.Name] = true })
	if len(seen) != 0 {
		t.Fatalf("expected no visited flags after rollback, got %v", seen)
	}

	// A later successful parse on a fresh baseline still works.
	if err := f.Parse([]string{"--a=+5"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{1, 5}; !reflect.DeepEqual(a, want) {
		t.Fatalf("expected %v, got %v", want, a)
	}
}
