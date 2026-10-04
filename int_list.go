package pflag

import (
	"fmt"
	"strconv"
	"strings"
)

// -- intList Value

// IntList is a registerable list of int values. Unlike IntSlice, every
// occurrence on the command line describes an explicit operation on the
// list instead of always appending:
//
//   - "=1,2" replaces the whole list with the comma-separated values. A
//     value without an operator ("1,2") also replaces, so "=" and the
//     empty string both replace the list with an empty one.
//   - "+3" appends the value 3 to the end of the list, preserving order.
//   - "-1" removes every element equal to 1 from the list, preserving the
//     order of the remaining elements; "-1,2" removes both values.
//
// Repeated occurrences are executed in command-line order. The payload
// keeps its own sign, making negative integers unambiguous: "+-3" appends
// -3 and "--3" removes all -3 elements, while "-3" removes positive 3.
//
// The default value, or any value assigned before parsing, is the
// baseline. All occurrences within a single Parse call form one atomic
// transaction: if any occurrence fails to parse, the flag reverts to that
// parse's baseline and Changed is left as it was before the parse. Each
// successful Parse starts its next baseline from the committed value.
// Calling Set outside of Parse applies the operation immediately, with no
// rollback.
type intListValue struct {
	value    *[]int
	baseline []int // baseline of the ongoing parse, nil outside a parse
}

func newIntListValue(val []int, p *[]int) *intListValue {
	il := new(intListValue)
	il.value = p
	*il.value = cloneIntSlice(val)
	return il
}

// BeginTransaction implements transactionalValue: the current list is the
// baseline that a failed parse rolls back to.
func (s *intListValue) BeginTransaction() {
	s.baseline = cloneIntSlice(*s.value)
}

// RollbackTransaction implements transactionalValue.
func (s *intListValue) RollbackTransaction() {
	*s.value = cloneIntSlice(s.baseline)
	s.baseline = nil
}

func cloneIntSlice(in []int) []int {
	out := make([]int, len(in))
	copy(out, in)
	return out
}

// parseIntListOperation parses one command-line occurrence into an operator
// ('=' replace, '+' append, '-' remove) and its comma-separated int values.
// All values are validated before the caller mutates the list.
func parseIntListOperation(val string) (op byte, out []int, err error) {
	op = '='
	body := val
	if len(val) > 0 {
		switch val[0] {
		case '=', '+', '-':
			op = val[0]
			// The payload keeps its own sign so signed integers are
			// unambiguous: "+-3" appends -3 and "--3" removes every -3
			// element, while "-3" removes 3. Atoi accepts "+3" as well,
			// which stays an append of 3.
			body = val[1:]
		}
	}

	// An empty body is only meaningful for replacement ("" or "="): it
	// yields an empty list. Appending or removing without a value is an
	// error.
	if body == "" {
		if op != '=' {
			return 0, nil, fmt.Errorf("%q: %q operation requires an integer value", val, string(op))
		}
		return op, []int{}, nil
	}

	parts := strings.Split(body, ",")
	out = make([]int, len(parts))
	for i, p := range parts {
		out[i], err = strconv.Atoi(p)
		if err != nil {
			// Keep the strconv error wording used by the other int flags.
			return 0, nil, err
		}
	}
	return op, out, nil
}

func (s *intListValue) Set(val string) error {
	op, values, err := parseIntListOperation(val)
	if err != nil {
		return err
	}

	current := *s.value
	switch op {
	case '=':
		current = cloneIntSlice(values)
	case '+':
		current = append(cloneIntSlice(current), values...)
	case '-':
		filtered := make([]int, 0, len(current))
		for _, v := range current {
			remove := false
			for _, target := range values {
				if v == target {
					remove = true
					break
				}
			}
			if !remove {
				filtered = append(filtered, v)
			}
		}
		current = filtered
	}
	*s.value = current
	return nil
}

func (s *intListValue) Type() string {
	return "intList"
}

func (s *intListValue) String() string {
	out := make([]string, len(*s.value))
	for i, d := range *s.value {
		out[i] = strconv.Itoa(d)
	}
	return "[" + strings.Join(out, ",") + "]"
}

func intListConv(val string) (interface{}, error) {
	val = strings.Trim(val, "[]")
	// Empty string represents an empty list.
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

// GetIntList returns the []int value of a flag with the given name.
func (f *FlagSet) GetIntList(name string) ([]int, error) {
	val, err := f.getFlagType(name, "intList", intListConv)
	if err != nil {
		return []int{}, err
	}
	return val.([]int), nil
}

// IntListVar defines an intList flag with the specified name, default value,
// and usage string. The argument p points to a []int variable in which to
// store the value of the flag.
//
// Each command-line occurrence is one list operation, applied in order:
// "=1,2" replaces the list (an empty "=" clears it), "+3" appends, and
// "-1" removes every element equal to 1. A bare value without an operator
// also replaces. Negative integers need an explicit operator: "+-3"
// appends -3 and "--3" removes all -3 elements. All occurrences of one
// Parse call are atomic: any invalid occurrence rolls the whole flag back
// to the value it had when that Parse started, and Changed reports the
// flag as changed only after a Parse that applied at least one operation
// successfully.
func (f *FlagSet) IntListVar(p *[]int, name string, value []int, usage string) {
	f.VarP(newIntListValue(value, p), name, "", usage)
}

// IntListVarP is like IntListVar, but accepts a shorthand letter that can
// be used after a single dash.
func (f *FlagSet) IntListVarP(p *[]int, name, shorthand string, value []int, usage string) {
	f.VarP(newIntListValue(value, p), name, shorthand, usage)
}

// IntListVar defines an intList flag with the specified name, default
// value, and usage string. The argument p points to a []int variable in
// which to store the value of the flag.
func IntListVar(p *[]int, name string, value []int, usage string) {
	CommandLine.VarP(newIntListValue(value, p), name, "", usage)
}

// IntListVarP is like IntListVar, but accepts a shorthand letter that can
// be used after a single dash.
func IntListVarP(p *[]int, name, shorthand string, value []int, usage string) {
	CommandLine.VarP(newIntListValue(value, p), name, shorthand, usage)
}

// IntList defines an intList flag with the specified name, default value,
// and usage string. The return value is the address of a []int variable
// that stores the value of the flag.
func (f *FlagSet) IntList(name string, value []int, usage string) *[]int {
	p := []int{}
	f.IntListVarP(&p, name, "", value, usage)
	return &p
}

// IntListP is like IntList, but accepts a shorthand letter that can be
// used after a single dash.
func (f *FlagSet) IntListP(name, shorthand string, value []int, usage string) *[]int {
	p := []int{}
	f.IntListVarP(&p, name, shorthand, value, usage)
	return &p
}

// IntList defines an intList flag with the specified name, default value,
// and usage string. The return value is the address of a []int variable
// that stores the value of the flag.
func IntList(name string, value []int, usage string) *[]int {
	return CommandLine.IntListP(name, "", value, usage)
}

// IntListP is like IntList, but accepts a shorthand letter that can be
// used after a single dash.
func IntListP(name, shorthand string, value []int, usage string) *[]int {
	return CommandLine.IntListP(name, shorthand, value, usage)
}
