package pflag

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// This file implements a registrable key=value group flag.
//
// Syntax (a single flag occurrence carries the whole group):
//
//	--group 'port=80,tag=a,tag=b'
//
//   - Pairs are separated by commas; key and value are separated by the
//     first '='.
//   - A backslash escapes the next byte, so 'tag=a\,b' keeps the comma
//     inside the value ('\\' yields a literal backslash, '\=' a literal
//     '='). Escapes are honored while splitting, so separators inside
//     escaped values are never treated as delimiters.
//   - Accepted keys, their value types and cardinality are declared up
//     front via a KeyValueGroupSpec.
//
// Semantics:
//
//   - The whole input is parsed and validated first; the combined object
//     is published (atomically replacing the previous one) only when the
//     entire parse succeeds. Unknown keys, duplicated single-value keys
//     and conversion errors are collected and reported together.
//   - On failure the previously published object (the baseline) is left
//     untouched, and Flag.Changed is not set by a failed Set.
//   - Multi-value keys preserve input order.

// FieldKind describes how the raw string of a declared field is converted.
type FieldKind int

const (
	// StringField accepts any string.
	StringField FieldKind = iota
	// IntField converts with strconv.ParseInt(s, 0, 64).
	IntField
	// FloatField converts with strconv.ParseFloat(s, 64).
	FloatField
	// BoolField converts with strconv.ParseBool.
	BoolField
	// DurationField converts with time.ParseDuration.
	DurationField
)

func (k FieldKind) String() string {
	switch k {
	case StringField:
		return "string"
	case IntField:
		return "int"
	case FloatField:
		return "float"
	case BoolField:
		return "bool"
	case DurationField:
		return "duration"
	}
	return fmt.Sprintf("FieldKind(%d)", int(k))
}

// FieldSpec declares one accepted key of a KeyValueGroup flag.
type FieldSpec struct {
	// Kind determines how the raw value is converted.
	Kind FieldKind
	// Multi allows the key to appear multiple times; values keep input
	// order and are published as a typed slice.
	Multi bool
}

// KeyValueGroupSpec maps accepted key names to their field declarations.
type KeyValueGroupSpec map[string]FieldSpec

// KeyValueGroup is the immutable object published by a keyValueGroupValue
// after a fully successful Set. Single-value keys hold a scalar (string,
// int64, float64, bool or time.Duration, per the declared FieldKind);
// multi-value keys hold the matching typed slice ([]string, []int64,
// []float64, []bool or []time.Duration) in input order.
type KeyValueGroup map[string]interface{}

// Lookup returns the raw published value for key.
func (g KeyValueGroup) Lookup(key string) (interface{}, bool) {
	v, ok := g[key]
	return v, ok
}

func (g KeyValueGroup) scalar(key string, dst interface{}) bool {
	v, ok := g[key]
	if !ok {
		return false
	}
	switch p := dst.(type) {
	case *string:
		x, ok := v.(string)
		*p = x
		return ok
	case *int64:
		x, ok := v.(int64)
		*p = x
		return ok
	case *float64:
		x, ok := v.(float64)
		*p = x
		return ok
	case *bool:
		x, ok := v.(bool)
		*p = x
		return ok
	case *time.Duration:
		x, ok := v.(time.Duration)
		*p = x
		return ok
	}
	return false
}

// String returns the single string value published for key.
func (g KeyValueGroup) String(key string) (string, bool) {
	var v string
	return v, g.scalar(key, &v)
}

// Int returns the single int64 value published for key.
func (g KeyValueGroup) Int(key string) (int64, bool) {
	var v int64
	return v, g.scalar(key, &v)
}

// Float returns the single float64 value published for key.
func (g KeyValueGroup) Float(key string) (float64, bool) {
	var v float64
	return v, g.scalar(key, &v)
}

// Bool returns the single bool value published for key.
func (g KeyValueGroup) Bool(key string) (bool, bool) {
	var v bool
	return v, g.scalar(key, &v)
}

// Duration returns the single time.Duration value published for key.
func (g KeyValueGroup) Duration(key string) (time.Duration, bool) {
	var v time.Duration
	return v, g.scalar(key, &v)
}

// Strings returns the ordered values of a multi string key.
func (g KeyValueGroup) Strings(key string) ([]string, bool) {
	v, ok := g[key]
	if !ok {
		return nil, false
	}
	s, ok := v.([]string)
	return s, ok
}

// Ints returns the ordered values of a multi int key.
func (g KeyValueGroup) Ints(key string) ([]int64, bool) {
	v, ok := g[key]
	if !ok {
		return nil, false
	}
	s, ok := v.([]int64)
	return s, ok
}

// Floats returns the ordered values of a multi float key.
func (g KeyValueGroup) Floats(key string) ([]float64, bool) {
	v, ok := g[key]
	if !ok {
		return nil, false
	}
	s, ok := v.([]float64)
	return s, ok
}

// Bools returns the ordered values of a multi bool key.
func (g KeyValueGroup) Bools(key string) ([]bool, bool) {
	v, ok := g[key]
	if !ok {
		return nil, false
	}
	s, ok := v.([]bool)
	return s, ok
}

// Durations returns the ordered values of a multi duration key.
func (g KeyValueGroup) Durations(key string) ([]time.Duration, bool) {
	v, ok := g[key]
	if !ok {
		return nil, false
	}
	s, ok := v.([]time.Duration)
	return s, ok
}

// -- keyValueGroup Value
type keyValueGroupValue struct {
	spec    KeyValueGroupSpec
	value   *KeyValueGroup
	changed bool
}

func newKeyValueGroupValue(spec KeyValueGroupSpec, val KeyValueGroup, p *KeyValueGroup) *keyValueGroupValue {
	v := new(keyValueGroupValue)
	v.spec = spec
	v.value = p
	if val == nil {
		val = KeyValueGroup{}
	}
	*v.value = val
	return v
}

// Set parses the whole group string and, only if every pair is valid,
// atomically publishes the new combined object. On any error the
// previously published object is kept and all problems are reported
// together.
func (v *keyValueGroupValue) Set(raw string) error {
	group, errs := v.parse(raw)
	if len(errs) > 0 {
		return fmt.Errorf("invalid key=value group %q: %s", raw, strings.Join(errs, "; "))
	}
	*v.value = group
	v.changed = true
	return nil
}

// parse converts raw into a publishable group. It returns a nil group and
// the list of all problems found when anything is invalid.
func (v *keyValueGroupValue) parse(raw string) (KeyValueGroup, []string) {
	if raw == "" {
		return KeyValueGroup{}, nil
	}

	var errs []string
	vals := map[string][]interface{}{}

	for _, seg := range splitEscaped(raw, ',') {
		eq := indexUnescaped(seg, '=')
		if eq < 0 {
			errs = append(errs, fmt.Sprintf("%q is not in key=value form", unescape(seg)))
			continue
		}
		key := unescape(seg[:eq])
		rawVal := unescape(seg[eq+1:])

		spec, ok := v.spec[key]
		if !ok {
			errs = append(errs, fmt.Sprintf("unknown key %q", key))
			continue
		}
		converted, err := convertField(spec.Kind, rawVal)
		if err != nil {
			errs = append(errs, fmt.Sprintf("key %q: cannot convert %q to %s: %v", key, rawVal, spec.Kind, err))
			continue
		}
		if !spec.Multi && len(vals[key]) == 1 {
			errs = append(errs, fmt.Sprintf("key %q is single-value but was given more than once", key))
			continue
		}
		vals[key] = append(vals[key], converted)
	}
	if len(errs) > 0 {
		return nil, errs
	}

	group := make(KeyValueGroup, len(vals))
	for key, vs := range vals {
		spec := v.spec[key]
		if spec.Multi {
			group[key] = makeMultiValue(spec.Kind, vs)
		} else {
			group[key] = vs[0]
		}
	}
	return group, nil
}

func convertField(kind FieldKind, raw string) (interface{}, error) {
	switch kind {
	case StringField:
		return raw, nil
	case IntField:
		return strconv.ParseInt(raw, 0, 64)
	case FloatField:
		return strconv.ParseFloat(raw, 64)
	case BoolField:
		return strconv.ParseBool(raw)
	case DurationField:
		return time.ParseDuration(raw)
	}
	return nil, fmt.Errorf("unsupported field kind %d", int(kind))
}

func makeMultiValue(kind FieldKind, vals []interface{}) interface{} {
	switch kind {
	case IntField:
		out := make([]int64, len(vals))
		for i, v := range vals {
			out[i] = v.(int64)
		}
		return out
	case FloatField:
		out := make([]float64, len(vals))
		for i, v := range vals {
			out[i] = v.(float64)
		}
		return out
	case BoolField:
		out := make([]bool, len(vals))
		for i, v := range vals {
			out[i] = v.(bool)
		}
		return out
	case DurationField:
		out := make([]time.Duration, len(vals))
		for i, v := range vals {
			out[i] = v.(time.Duration)
		}
		return out
	default:
		out := make([]string, len(vals))
		for i, v := range vals {
			out[i] = v.(string)
		}
		return out
	}
}

func (v *keyValueGroupValue) Type() string {
	return "keyValueGroup"
}

func (v *keyValueGroupValue) String() string {
	g := *v.value
	if len(g) == 0 {
		return ""
	}
	keys := make([]string, 0, len(g))
	for k := range g {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, renderGroupPairs(k, g[k])...)
	}
	return strings.Join(parts, ",")
}

func renderGroupPairs(key string, val interface{}) []string {
	switch tv := val.(type) {
	case []string:
		parts := make([]string, len(tv))
		for i, e := range tv {
			parts[i] = escapeGroup(key) + "=" + escapeGroup(e)
		}
		return parts
	case []int64:
		parts := make([]string, len(tv))
		for i, e := range tv {
			parts[i] = escapeGroup(key) + "=" + strconv.FormatInt(e, 10)
		}
		return parts
	case []float64:
		parts := make([]string, len(tv))
		for i, e := range tv {
			parts[i] = escapeGroup(key) + "=" + strconv.FormatFloat(e, 'g', -1, 64)
		}
		return parts
	case []bool:
		parts := make([]string, len(tv))
		for i, e := range tv {
			parts[i] = escapeGroup(key) + "=" + strconv.FormatBool(e)
		}
		return parts
	case []time.Duration:
		parts := make([]string, len(tv))
		for i, e := range tv {
			parts[i] = escapeGroup(key) + "=" + e.String()
		}
		return parts
	case string:
		return []string{escapeGroup(key) + "=" + escapeGroup(tv)}
	default:
		return []string{escapeGroup(key) + "=" + fmt.Sprintf("%v", tv)}
	}
}

// splitEscaped splits s on unescaped occurrences of sep. Escape sequences
// are kept in the returned segments for a later unescape pass.
func splitEscaped(s string, sep byte) []string {
	var parts []string
	var cur []byte
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			cur = append(cur, '\\', c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == sep {
			parts = append(parts, string(cur))
			cur = cur[:0]
			continue
		}
		cur = append(cur, c)
	}
	if escaped {
		cur = append(cur, '\\')
	}
	return append(parts, string(cur))
}

// indexUnescaped returns the index of the first unescaped sep in s, or -1.
func indexUnescaped(s string, sep byte) int {
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == sep {
			return i
		}
	}
	return -1
}

// unescape resolves backslash escape sequences in s.
func unescape(s string) string {
	if strings.IndexByte(s, '\\') < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			b.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		b.WriteByte(c)
	}
	if escaped {
		b.WriteByte('\\')
	}
	return b.String()
}

var groupEscaper = strings.NewReplacer("\\", "\\\\", ",", "\\,", "=", "\\=")

// escapeGroup quotes the bytes that are significant in the group syntax.
func escapeGroup(s string) string {
	return groupEscaper.Replace(s)
}

// KeyValueGroupVar defines a key=value group flag with the given name,
// default value, field specification and usage string. The argument p
// points to the object that is atomically replaced on every fully
// successful Set.
func (f *FlagSet) KeyValueGroupVar(p *KeyValueGroup, name string, value KeyValueGroup, spec KeyValueGroupSpec, usage string) {
	f.VarP(newKeyValueGroupValue(spec, value, p), name, "", usage)
}

// KeyValueGroup defines a key=value group flag and returns a pointer to
// the published object.
func (f *FlagSet) KeyValueGroup(name string, value KeyValueGroup, spec KeyValueGroupSpec, usage string) *KeyValueGroup {
	p := new(KeyValueGroup)
	f.KeyValueGroupVar(p, name, value, spec, usage)
	return p
}

// KeyValueGroupVar defines a key=value group flag on the CommandLine
// FlagSet.
func KeyValueGroupVar(p *KeyValueGroup, name string, value KeyValueGroup, spec KeyValueGroupSpec, usage string) {
	CommandLine.VarP(newKeyValueGroupValue(spec, value, p), name, "", usage)
}

// Note: there is no package-level KeyValueGroup constructor because its
// name would collide with the KeyValueGroup type; use
// CommandLine.KeyValueGroup instead.

// GetKeyValueGroup returns the object currently published by the named
// key=value group flag.
func (f *FlagSet) GetKeyValueGroup(name string) (KeyValueGroup, error) {
	val, err := f.getFlagType(name, "keyValueGroup", nil)
	if err != nil {
		return nil, err
	}
	return *(val.(*keyValueGroupValue).value), nil
}

// GetKeyValueGroup returns the object currently published by the named
// key=value group flag on the CommandLine FlagSet.
func GetKeyValueGroup(name string) (KeyValueGroup, error) {
	return CommandLine.GetKeyValueGroup(name)
}
