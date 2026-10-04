package pflag

import (
	"fmt"
	"strconv"
	"strings"
)

// intListValue is a []int flag that supports in-place list operations on the
// command line. It is registered through the standard Value interface (see
// IntListVar and friends) and is intentionally distinct from IntSlice, which
// only replaces (on first occurrence) or appends (on later occurrences).
//
// # Operations
//
// The argument passed to Set is a single operation. The first character is
// the operator and the remainder is the operand:
//
//	=1,2   replace the whole list with [1, 2] (an empty operand, "=", clears it)
//	+3     append the integer 3
//	-1     delete every occurrence of the integer 1
//
// Repeated occurrences are applied in command-line order, so
//
//	--il=+3 --il=-1 --il=+2
//
// starting from a baseline of [1,2] yields [2,3,2].
//
// Negative integers must be wrapped in parentheses so they are not confused
// with the delete operator. For example, "+(-3)" appends -3, "-(-3)" deletes
// every -3, while "-3" deletes every 3.
//
// # Baseline and error atomicity
//
// The baseline of a Set call is the value the list held before that call:
// the environment/default value for the first occurrence, and the result of
// the previous successful Set for every later occurrence (whether in the same
// or a different Parse call). Each Set is atomic: the operation is applied to
// a local copy and the list is only updated after the whole operation has been
// parsed successfully. If any part of the operation is invalid the list is
// left exactly at its baseline and the error is returned. Changed is only set
// after a Set that parsed successfully; a failed Set never sets it.
type intListValue struct {
	value   *[]int
	changed bool
}

func newIntListValue(val []int, p *[]int) *intListValue {
	ilv := new(intListValue)
	ilv.value = p
	*ilv.value = val
	return ilv
}

// parseIntListToken parses a single integer token. A token is either a plain
// non-negative integer ("3") or a parenthesized integer ("(-3)"). Parentheses
// are the distinguishable syntax for negative integers, which would otherwise
// be ambiguous with the delete operator. When allowBareNegative is false a
// bare negative integer is rejected; this is used by the append and delete
// operations, where the sign could be mistaken for an operator.
func parseIntListToken(s string, allowBareNegative bool) (int, error) {
	if len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		n, err := strconv.Atoi(s[1 : len(s)-1])
		if err != nil {
			return 0, err
		}
		return n, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if !allowBareNegative && n < 0 {
		return 0, fmt.Errorf("negative integer %q must be parenthesized as (%s)", s, s)
	}
	return n, nil
}

// parseIntListOperand parses the comma-separated operand of the replace
// operation. An empty operand yields an empty list (this is the empty
// replacement, "=").
func parseIntListOperand(operand string) ([]int, error) {
	if operand == "" {
		return []int{}, nil
	}
	parts := strings.Split(operand, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := parseIntListToken(p, true)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

// parseIntListSingle parses the single-integer operand of the append and
// delete operations.
func parseIntListSingle(operand string) (int, error) {
	if operand == "" {
		return 0, fmt.Errorf("missing integer operand")
	}
	n, err := parseIntListToken(operand, false)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *intListValue) Set(val string) error {
	if len(val) == 0 {
		return fmt.Errorf("invalid intList value %q: expected an operation prefix (=, +, -)", val)
	}
	op := val[0]
	operand := val[1:]

	var result []int
	switch op {
	case '=':
		// Replace: parse the whole operand before touching the list.
		list, err := parseIntListOperand(operand)
		if err != nil {
			return err
		}
		result = list
	case '+':
		// Append: build the new list locally so a parse failure leaves the
		// baseline untouched.
		n, err := parseIntListSingle(operand)
		if err != nil {
			return err
		}
		result = make([]int, 0, len(*s.value)+1)
		result = append(result, *s.value...)
		result = append(result, n)
	case '-':
		// Delete: filter into a new list. The list elements are already ints,
		// so this cannot fail once the operand parses.
		n, err := parseIntListSingle(operand)
		if err != nil {
			return err
		}
		result = make([]int, 0, len(*s.value))
		for _, v := range *s.value {
			if v != n {
				result = append(result, v)
			}
		}
	default:
		return fmt.Errorf("invalid intList operation %q: expected one of '=', '+', '-'", string(op))
	}

	*s.value = result
	s.changed = true
	return nil
}

func (s *intListValue) Type() string {
	return "intList"
}

func (s *intListValue) String() string {
	out := make([]string, len(*s.value))
	for i, d := range *s.value {
		out[i] = fmt.Sprintf("%d", d)
	}
	return "[" + strings.Join(out, ",") + "]"
}

func intListConv(val string) (interface{}, error) {
	val = strings.Trim(val, "[]")
	// An empty string would cause a slice with one (empty) entry.
	if len(val) == 0 {
		return []int{}, nil
	}
	ss := strings.Split(val, ",")
	out := make([]int, len(ss))
	for i, d := range ss {
		var err error
		out[i], err = strconv.Atoi(d)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// GetIntList return the []int value of a flag with the given name
func (f *FlagSet) GetIntList(name string) ([]int, error) {
	val, err := f.getFlagType(name, "intList", intListConv)
	if err != nil {
		return []int{}, err
	}
	return val.([]int), nil
}

// IntListVar defines an intList flag with specified name, default value, and usage string.
// The argument p points to a []int variable in which to store the value of the flag.
//
// The flag value is a single operation: "=1,2" replaces the list, "+3" appends
// 3, and "-1" deletes every 1. Repeated occurrences run in command-line order.
// Negative integers must be parenthesized, e.g. "+(-3)". See the intListValue
// type documentation for the full syntax and error-atomicity semantics.
func (f *FlagSet) IntListVar(p *[]int, name string, value []int, usage string) {
	f.VarP(newIntListValue(value, p), name, "", usage)
}

// IntListVarP is like IntListVar, but accepts a shorthand letter that can be used after a single dash.
func (f *FlagSet) IntListVarP(p *[]int, name, shorthand string, value []int, usage string) {
	f.VarP(newIntListValue(value, p), name, shorthand, usage)
}

// IntListVar defines an intList flag with specified name, default value, and usage string.
// The argument p points to a []int variable in which to store the value of the flag.
func IntListVar(p *[]int, name string, value []int, usage string) {
	CommandLine.VarP(newIntListValue(value, p), name, "", usage)
}

// IntListVarP is like IntListVar, but accepts a shorthand letter that can be used after a single dash.
func IntListVarP(p *[]int, name, shorthand string, value []int, usage string) {
	CommandLine.VarP(newIntListValue(value, p), name, shorthand, usage)
}

// IntList defines a []int flag with specified name, default value, and usage string.
// The return value is the address of a []int variable that stores the value of the flag.
func (f *FlagSet) IntList(name string, value []int, usage string) *[]int {
	p := []int{}
	f.IntListVarP(&p, name, "", value, usage)
	return &p
}

// IntListP is like IntList, but accepts a shorthand letter that can be used after a single dash.
func (f *FlagSet) IntListP(name, shorthand string, value []int, usage string) *[]int {
	p := []int{}
	f.IntListVarP(&p, name, shorthand, value, usage)
	return &p
}

// IntList defines a []int flag with specified name, default value, and usage string.
// The return value is the address of a []int variable that stores the value of the flag.
func IntList(name string, value []int, usage string) *[]int {
	return CommandLine.IntListP(name, "", value, usage)
}

// IntListP is like IntList, but accepts a shorthand letter that can be used after a single dash.
func IntListP(name, shorthand string, value []int, usage string) *[]int {
	return CommandLine.IntListP(name, shorthand, value, usage)
}
