package pflag

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// KeyValueGroup is a registerable pflag Value that parses repeated key=value
// pairs into a typed group of fields. It is registered with a FlagSet via
// Var (or KeyValueGroupVar) and converts each value according to the declared
// field type.
//
// Syntax
//
//	--<flag> key=value[,key=value...]
//
// Fields are declared by name together with a typed pointer. Single-value
// fields may appear at most once; multi-value fields (such as
// StringSliceField) accumulate every occurrence in input order. A value may
// contain the separator comma or the equals sign by escaping it with a
// backslash:
//
//	tag=a,b      -> tag: ["a", "b"]
//	tag=a\,b     -> tag: ["a,b"]
//	tag=a\=b     -> tag: ["a=b"]
//	port=80      -> port: 80
//
// Parsing is transactional: the whole argument is parsed and validated
// before anything is published. Unknown keys, repeated single-value keys, and
// type conversion errors are collected and reported together; on error the
// previously published object and the flag Changed state are left untouched.
// Multi-value fields keep their input order across repeated --<flag>
// invocations.
//
// Example:
//
//	type Server struct {
//		Port int
//		Tags []string
//	}
//	cfg := &Server{}
//	group := pflag.NewKeyValueGroup().
//		IntField("port", &cfg.Port).
//		StringSliceField("tag", &cfg.Tags)
//	flags.Var(group, "group", "key=value group, e.g. port=80,tag=a,tag=b")
type KeyValueGroup struct {
	fields  []*kvGroupField
	byName  map[string]*kvGroupField
	changed bool
}

// kvGroupField describes a single declared key in a KeyValueGroup.
type kvGroupField struct {
	name      string
	multi     bool
	conv      func(string) (interface{}, error)
	assign    func(interface{})
	clear     func()
	format    func() []string
	setByUser bool
}

// NewKeyValueGroup returns an empty KeyValueGroup. Declare fields with the
// typed field methods before registering the group with a FlagSet.
func NewKeyValueGroup() *KeyValueGroup {
	return &KeyValueGroup{byName: map[string]*kvGroupField{}}
}

func (g *KeyValueGroup) addField(name string, multi bool,
	conv func(string) (interface{}, error),
	assign func(interface{}), clear func(), format func() []string) *KeyValueGroup {
	if name == "" {
		panic("pflag: key=value group field name must not be empty")
	}
	if _, exists := g.byName[name]; exists {
		panic(fmt.Sprintf("pflag: key=value group field %q declared twice", name))
	}
	f := &kvGroupField{name: name, multi: multi, conv: conv, assign: assign, clear: clear, format: format}
	g.fields = append(g.fields, f)
	g.byName[name] = f
	return g
}

// StringField declares a single-value string field bound to p.
func (g *KeyValueGroup) StringField(name string, p *string) *KeyValueGroup {
	return g.addField(name, false,
		func(s string) (interface{}, error) { return s, nil },
		func(v interface{}) { *p = v.(string) },
		nil,
		func() []string { return []string{*p} })
}

// IntField declares a single-value int field bound to p.
func (g *KeyValueGroup) IntField(name string, p *int) *KeyValueGroup {
	return g.addField(name, false,
		func(s string) (interface{}, error) { return strconv.Atoi(s) },
		func(v interface{}) { *p = v.(int) },
		nil,
		func() []string { return []string{strconv.Itoa(*p)} })
}

// Int64Field declares a single-value int64 field bound to p.
func (g *KeyValueGroup) Int64Field(name string, p *int64) *KeyValueGroup {
	return g.addField(name, false,
		func(s string) (interface{}, error) { return strconv.ParseInt(s, 0, 64) },
		func(v interface{}) { *p = v.(int64) },
		nil,
		func() []string { return []string{strconv.FormatInt(*p, 10)} })
}

// UintField declares a single-value uint field bound to p.
func (g *KeyValueGroup) UintField(name string, p *uint) *KeyValueGroup {
	return g.addField(name, false,
		func(s string) (interface{}, error) {
			v, err := strconv.ParseUint(s, 0, 64)
			return uint(v), err
		},
		func(v interface{}) { *p = v.(uint) },
		nil,
		func() []string { return []string{strconv.FormatUint(uint64(*p), 10)} })
}

// Uint64Field declares a single-value uint64 field bound to p.
func (g *KeyValueGroup) Uint64Field(name string, p *uint64) *KeyValueGroup {
	return g.addField(name, false,
		func(s string) (interface{}, error) { return strconv.ParseUint(s, 0, 64) },
		func(v interface{}) { *p = v.(uint64) },
		nil,
		func() []string { return []string{strconv.FormatUint(*p, 10)} })
}

// BoolField declares a single-value bool field bound to p.
func (g *KeyValueGroup) BoolField(name string, p *bool) *KeyValueGroup {
	return g.addField(name, false,
		func(s string) (interface{}, error) { return strconv.ParseBool(s) },
		func(v interface{}) { *p = v.(bool) },
		nil,
		func() []string { return []string{strconv.FormatBool(*p)} })
}

// Float64Field declares a single-value float64 field bound to p.
func (g *KeyValueGroup) Float64Field(name string, p *float64) *KeyValueGroup {
	return g.addField(name, false,
		func(s string) (interface{}, error) { return strconv.ParseFloat(s, 64) },
		func(v interface{}) { *p = v.(float64) },
		nil,
		func() []string { return []string{strconv.FormatFloat(*p, 'g', -1, 64)} })
}

// DurationField declares a single-value time.Duration field bound to p.
func (g *KeyValueGroup) DurationField(name string, p *time.Duration) *KeyValueGroup {
	return g.addField(name, false,
		func(s string) (interface{}, error) { return time.ParseDuration(s) },
		func(v interface{}) { *p = v.(time.Duration) },
		nil,
		func() []string { return []string{p.String()} })
}

// StringSliceField declares a multi-value string slice field bound to p.
// Every occurrence is appended in input order; the first successful set
// replaces any default value.
func (g *KeyValueGroup) StringSliceField(name string, p *[]string) *KeyValueGroup {
	return g.addField(name, true,
		func(s string) (interface{}, error) { return s, nil },
		func(v interface{}) { *p = append(*p, v.(string)) },
		func() { *p = nil },
		func() []string { return *p })
}

// backslash is the escape character used in key=value groups.
const backslash = byte(92)

// splitEscaped splits s on sep, honoring backslash escapes. An escaped
// character is kept verbatim (including the backslash) so it can be
// unescaped later.
func splitEscaped(s string, sep byte) []string {
	parts := make([]string, 0, 1)
	var buf strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == backslash && i+1 < len(s) {
			buf.WriteByte(s[i])
			buf.WriteByte(s[i+1])
			i++
			continue
		}
		if s[i] == sep {
			parts = append(parts, buf.String())
			buf.Reset()
			continue
		}
		buf.WriteByte(s[i])
	}
	parts = append(parts, buf.String())
	return parts
}

// splitFirstUnescaped splits s at the first unescaped sep.
func splitFirstUnescaped(s string, sep byte) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == backslash && i+1 < len(s) {
			i++
			continue
		}
		if s[i] == sep {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

// unescapeKV removes backslash escapes from a key or value.
func unescapeKV(s string) (string, error) {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == backslash {
			if i+1 >= len(s) {
				return "", fmt.Errorf("key=value group: unterminated escape at position %d", i)
			}
			b.WriteByte(s[i+1])
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String(), nil
}

// escapeKV escapes a key or value for round-tripping through String.
func escapeKV(s string) string {
	bs := string([]byte{backslash})
	s = strings.ReplaceAll(s, bs, bs+bs)
	s = strings.ReplaceAll(s, ",", bs+",")
	s = strings.ReplaceAll(s, "=", bs+"=")
	return s
}

// Set implements the Value interface. It parses the whole argument and only
// publishes the result if every key and value is valid.
func (g *KeyValueGroup) Set(val string) error {
	if val == "" {
		return fmt.Errorf("%q must be formatted as key=value", val)
	}

	type pending struct {
		field *kvGroupField
		value interface{}
	}
	var batch []pending
	seen := make(map[string]bool)
	var problems []error

	for _, pair := range splitEscaped(val, ',') {
		rawKey, rawVal, ok := splitFirstUnescaped(pair, '=')
		if !ok {
			problems = append(problems, fmt.Errorf("%q must be formatted as key=value", pair))
			continue
		}
		key, err := unescapeKV(rawKey)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		rawValue, err := unescapeKV(rawVal)
		if err != nil {
			problems = append(problems, err)
			continue
		}

		f, exists := g.byName[key]
		if !exists {
			problems = append(problems, fmt.Errorf("unknown key %q", key))
			continue
		}
		if !f.multi {
			if seen[key] {
				problems = append(problems, fmt.Errorf("key %q may not be repeated", key))
				continue
			}
			if f.setByUser {
				problems = append(problems, fmt.Errorf("key %q may not be repeated", key))
				continue
			}
			seen[key] = true
		}

		converted, err := f.conv(rawValue)
		if err != nil {
			problems = append(problems, fmt.Errorf("invalid value %q for key %q: %v", rawValue, key, err))
			continue
		}
		batch = append(batch, pending{field: f, value: converted})
	}

	if len(problems) > 0 {
		return &KeyValueGroupError{Problems: problems}
	}

	for _, p := range batch {
		f := p.field
		if f.multi && !f.setByUser {
			f.clear()
		}
		f.assign(p.value)
		f.setByUser = true
	}
	g.changed = true
	return nil
}

// Type implements the Value interface.
func (g *KeyValueGroup) Type() string { return "keyValueGroup" }

// String implements the Value interface and renders the current published
// object.
func (g *KeyValueGroup) String() string {
	parts := make([]string, 0, len(g.fields))
	for _, f := range g.fields {
		for _, v := range f.format() {
			parts = append(parts, escapeKV(f.name)+"="+escapeKV(v))
		}
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// KeyValueGroupError reports one or more problems found while parsing a
// key=value group.
type KeyValueGroupError struct {
	Problems []error
}

// Error implements error.
func (e *KeyValueGroupError) Error() string {
	var b strings.Builder
	b.WriteString("invalid key=value group: ")
	for i, p := range e.Problems {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(p.Error())
	}
	return b.String()
}

// KeyValueGroupVar registers a KeyValueGroup with the FlagSet.
func (f *FlagSet) KeyValueGroupVar(p *KeyValueGroup, name, usage string) {
	f.VarP(p, name, "", usage)
}

// KeyValueGroupVarP registers a KeyValueGroup with the FlagSet and accepts a
// shorthand letter.
func (f *FlagSet) KeyValueGroupVarP(p *KeyValueGroup, name, shorthand, usage string) {
	f.VarP(p, name, shorthand, usage)
}

// KeyValueGroupVar registers a KeyValueGroup with the CommandLine FlagSet.
func KeyValueGroupVar(p *KeyValueGroup, name, usage string) {
	CommandLine.VarP(p, name, "", usage)
}

// KeyValueGroupVarP registers a KeyValueGroup with the CommandLine FlagSet
// and accepts a shorthand letter.
func KeyValueGroupVarP(p *KeyValueGroup, name, shorthand, usage string) {
	CommandLine.VarP(p, name, shorthand, usage)
}
