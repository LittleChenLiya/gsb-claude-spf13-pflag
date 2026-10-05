package pflag

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func kvgTestSpec() KeyValueGroupSpec {
	return KeyValueGroupSpec{
		"port":    {Kind: IntField},
		"tag":     {Kind: StringField, Multi: true},
		"rate":    {Kind: FloatField},
		"debug":   {Kind: BoolField},
		"timeout": {Kind: DurationField},
	}
}

// port=80 and tag=a,tag=b compose into one object; Changed reports the
// successful publication.
func TestKeyValueGroupCompose(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	g := f.KeyValueGroup("group", nil, kvgTestSpec(), "key=value group")

	if err := f.Parse([]string{"--group", "port=80,tag=a,tag=b"}); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !f.Changed("group") {
		t.Error("Changed should be true after a successful parse")
	}

	port, ok := g.Int("port")
	if !ok || port != 80 {
		t.Errorf("port = %v, %v; want 80, true", port, ok)
	}
	tags, ok := g.Strings("tag")
	if !ok || !reflect.DeepEqual(tags, []string{"a", "b"}) {
		t.Errorf("tag = %v, %v; want [a b], true", tags, ok)
	}

	got, err := f.GetKeyValueGroup("group")
	if err != nil {
		t.Fatalf("GetKeyValueGroup failed: %v", err)
	}
	if !reflect.DeepEqual(got, *g) {
		t.Errorf("GetKeyValueGroup = %v; want %v", got, *g)
	}
}

// A failed Set (port=bad) leaves the previously published object and
// Changed untouched.
func TestKeyValueGroupFailureRestoresBaseline(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	g := f.KeyValueGroup("group", nil, kvgTestSpec(), "key=value group")

	if err := f.Parse([]string{"--group", "port=80,tag=a,tag=b"}); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	before := *g

	if err := f.Set("group", "port=bad"); err == nil {
		t.Fatal("Set with port=bad should fail")
	}
	if !reflect.DeepEqual(*g, before) {
		t.Errorf("published object changed after failed Set: %v -> %v", before, *g)
	}
	if !f.Changed("group") {
		t.Error("Changed should still be true (last publication succeeded)")
	}
	port, _ := g.Int("port")
	if port != 80 {
		t.Errorf("port = %d; want 80 (baseline restored)", port)
	}
}

// A failed first parse keeps the default object, Changed stays false and
// independent flags keep their default semantics.
func TestKeyValueGroupFailedFirstParseKeepsDefault(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	def := KeyValueGroup{"port": int64(1)}
	g := f.KeyValueGroup("group", def, kvgTestSpec(), "key=value group")
	name := f.String("name", "default", "independent flag")

	if err := f.Parse([]string{"--group", "port=bad"}); err == nil {
		t.Fatal("Parse with port=bad should fail")
	}
	if f.Changed("group") {
		t.Error("Changed must stay false: nothing was ever published")
	}
	port, ok := g.Int("port")
	if !ok || port != 1 {
		t.Errorf("port = %v, %v; want the default 1, true", port, ok)
	}
	if *name != "default" || f.Changed("name") {
		t.Errorf("independent flag affected: name=%q changed=%v", *name, f.Changed("name"))
	}
}

// Unknown keys, duplicated single-value keys and conversion errors are
// collected and reported together; nothing is published.
func TestKeyValueGroupAggregatedErrors(t *testing.T) {
	var g KeyValueGroup
	v := newKeyValueGroupValue(kvgTestSpec(), nil, &g)

	err := v.Set("port=1,port=2,bogus=x,rate=abc")
	if err == nil {
		t.Fatal("expected aggregated error")
	}
	msg := err.Error()
	for _, want := range []string{
		`unknown key "bogus"`,
		`key "port" is single-value but was given more than once`,
		`key "rate": cannot convert "abc" to float`,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not contain %q", msg, want)
		}
	}
	if len(g) != 0 {
		t.Errorf("nothing may be published on failure, got %v", g)
	}
	if v.changed {
		t.Error("internal changed flag must stay false after a failed Set")
	}
}

// Escaped separators stay inside values: the comma in tag=a\,b must not
// split the pair, \\ yields a literal backslash and \= a literal '='.
func TestKeyValueGroupEscapes(t *testing.T) {
	var g KeyValueGroup
	v := newKeyValueGroupValue(kvgTestSpec(), nil, &g)

	if err := v.Set(`tag=a\,b,tag=c\\d,tag=e\=f,port=80`); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	tags, ok := g.Strings("tag")
	want := []string{"a,b", `c\d`, "e=f"}
	if !ok || !reflect.DeepEqual(tags, want) {
		t.Errorf("tag = %q, %v; want %q, true", tags, ok, want)
	}
	if port, _ := g.Int("port"); port != 80 {
		t.Errorf("port = %d; want 80", port)
	}
}

// Multi-value keys keep input order.
func TestKeyValueGroupMultiOrder(t *testing.T) {
	var g KeyValueGroup
	v := newKeyValueGroupValue(kvgTestSpec(), nil, &g)

	if err := v.Set("tag=z,tag=a,tag=m"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	tags, _ := g.Strings("tag")
	if !reflect.DeepEqual(tags, []string{"z", "a", "m"}) {
		t.Errorf("tag = %v; want [z a m] (input order)", tags)
	}
}

// Values are converted per the declared field type.
func TestKeyValueGroupFieldTypes(t *testing.T) {
	var g KeyValueGroup
	v := newKeyValueGroupValue(kvgTestSpec(), nil, &g)

	if err := v.Set("port=0x10,debug=true,rate=0.5,timeout=1h30m"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if port, _ := g.Int("port"); port != 16 {
		t.Errorf("port = %d; want 16", port)
	}
	if debug, _ := g.Bool("debug"); !debug {
		t.Error("debug = false; want true")
	}
	if rate, _ := g.Float("rate"); rate != 0.5 {
		t.Errorf("rate = %v; want 0.5", rate)
	}
	if timeout, _ := g.Duration("timeout"); timeout != 90*time.Minute {
		t.Errorf("timeout = %v; want 1h30m", timeout)
	}
}

// String() renders the published object in the group syntax, escapes
// significant bytes and round-trips.
func TestKeyValueGroupStringRoundTrip(t *testing.T) {
	var g KeyValueGroup
	v := newKeyValueGroupValue(kvgTestSpec(), nil, &g)

	if err := v.Set(`tag=a\,b,tag=c,port=80`); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	s := v.String()
	if want := `port=80,tag=a\,b,tag=c`; s != want {
		t.Errorf("String() = %q; want %q", s, want)
	}

	var g2 KeyValueGroup
	v2 := newKeyValueGroupValue(kvgTestSpec(), nil, &g2)
	if err := v2.Set(s); err != nil {
		t.Fatalf("re-parse of String() output failed: %v", err)
	}
	if !reflect.DeepEqual(g2, g) {
		t.Errorf("round trip mismatch: %v vs %v", g2, g)
	}
}

// An empty input publishes an empty group.
func TestKeyValueGroupEmpty(t *testing.T) {
	var g KeyValueGroup
	v := newKeyValueGroupValue(kvgTestSpec(), KeyValueGroup{"port": int64(1)}, &g)

	if err := v.Set(""); err != nil {
		t.Fatalf(`Set("") failed: %v`, err)
	}
	if len(g) != 0 {
		t.Errorf("group = %v; want empty", g)
	}
}

// A malformed pair (no '=') is reported and nothing is published.
func TestKeyValueGroupMalformedPair(t *testing.T) {
	var g KeyValueGroup
	v := newKeyValueGroupValue(kvgTestSpec(), nil, &g)

	err := v.Set("port")
	if err == nil || !strings.Contains(err.Error(), "key=value") {
		t.Errorf("err = %v; want a key=value form error", err)
	}
	if len(g) != 0 {
		t.Errorf("nothing may be published on failure, got %v", g)
	}
}

// Independent flags keep their default semantics when a group flag is
// parsed successfully alongside them.
func TestKeyValueGroupIndependentFlags(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	g := f.KeyValueGroup("group", nil, kvgTestSpec(), "key=value group")
	name := f.String("name", "default", "independent flag")
	verbose := f.Bool("verbose", false, "independent bool")

	if err := f.Parse([]string{"--group", "port=80", "--name", "x", "--verbose"}); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if *name != "x" || !f.Changed("name") {
		t.Errorf("name = %q changed=%v; want x, true", *name, f.Changed("name"))
	}
	if !*verbose || !f.Changed("verbose") {
		t.Errorf("verbose = %v changed=%v; want true, true", *verbose, f.Changed("verbose"))
	}
	if port, _ := g.Int("port"); port != 80 {
		t.Errorf("port = %d; want 80", port)
	}
}
