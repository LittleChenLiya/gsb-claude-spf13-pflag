package pflag

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// kvBS is the backslash character, used to build escaped key=value strings
// without embedding a literal backslash in the source.
var kvBS = string([]byte{92})

type kvGroupServer struct {
	Port int
	Tags []string
}

func newKVGroupServer() *kvGroupServer {
	return &kvGroupServer{}
}

func newKVGroupFlagSet(cfg *kvGroupServer) *FlagSet {
	fs := NewFlagSet("test", ContinueOnError)
	g := NewKeyValueGroup().
		IntField("port", &cfg.Port).
		StringSliceField("tag", &cfg.Tags)
	fs.Var(g, "group", "key=value group, e.g. port=80,tag=a,tag=b")
	return fs
}

func TestKeyValueGroupBasic(t *testing.T) {
	cfg := newKVGroupServer()
	fs := newKVGroupFlagSet(cfg)

	if err := fs.Parse([]string{"--group", "port=80,tag=a,tag=b"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Port != 80 {
		t.Errorf("Port = %d, want 80", cfg.Port)
	}
	if len(cfg.Tags) != 2 || cfg.Tags[0] != "a" || cfg.Tags[1] != "b" {
		t.Errorf("Tags = %v, want [a b]", cfg.Tags)
	}
	if !fs.Changed("group") {
		t.Error("Changed(group) = false, want true")
	}
}

func TestKeyValueGroupRepeated(t *testing.T) {
	cfg := newKVGroupServer()
	fs := newKVGroupFlagSet(cfg)

	if err := fs.Parse([]string{"--group", "port=80", "--group", "tag=a", "--group", "tag=b"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Port != 80 {
		t.Errorf("Port = %d, want 80", cfg.Port)
	}
	if len(cfg.Tags) != 2 || cfg.Tags[0] != "a" || cfg.Tags[1] != "b" {
		t.Errorf("Tags = %v, want [a b]", cfg.Tags)
	}
}

func TestKeyValueGroupFailureRestoresBaseline(t *testing.T) {
	cfg := newKVGroupServer()
	fs := newKVGroupFlagSet(cfg)

	if err := fs.Parse([]string{"--group", "port=80"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Port != 80 {
		t.Fatalf("Port = %d, want 80", cfg.Port)
	}
	if !fs.Changed("group") {
		t.Fatal("Changed(group) = false, want true")
	}

	// A failed parse must not touch the published object or Changed state.
	err := fs.Parse([]string{"--group", "port=bad"})
	if err == nil {
		t.Fatal("Parse(port=bad) = nil, want error")
	}
	if cfg.Port != 80 {
		t.Errorf("Port = %d after failed parse, want 80 (baseline restored)", cfg.Port)
	}
	if !fs.Changed("group") {
		t.Error("Changed(group) = false after failed parse, want true")
	}

	// A subsequent parse with the same single-value key is also rejected.
	if err := fs.Parse([]string{"--group", "port=443"}); err == nil {
		t.Fatal("Parse(port=443) = nil, want error (repeated key)")
	}
	if cfg.Port != 80 {
		t.Errorf("Port = %d, want 80 (baseline preserved)", cfg.Port)
	}
}

func TestKeyValueGroupUnknownKey(t *testing.T) {
	cfg := newKVGroupServer()
	fs := newKVGroupFlagSet(cfg)

	err := fs.Parse([]string{"--group", "port=80,unknown=x"})
	if err == nil {
		t.Fatal("Parse with unknown key = nil, want error")
	}
	if !strings.Contains(err.Error(), "unknown key") {
		t.Errorf("error = %q, want unknown key report", err.Error())
	}
	// Nothing published: baseline (zero) preserved.
	if cfg.Port != 0 {
		t.Errorf("Port = %d, want 0 (nothing published)", cfg.Port)
	}
	if fs.Changed("group") {
		t.Error("Changed(group) = true, want false")
	}
}

func TestKeyValueGroupRepeatedSingleKey(t *testing.T) {
	cfg := newKVGroupServer()
	fs := newKVGroupFlagSet(cfg)

	err := fs.Parse([]string{"--group", "port=80,port=90"})
	if err == nil {
		t.Fatal("Parse with repeated port = nil, want error")
	}
	if !strings.Contains(err.Error(), "may not be repeated") {
		t.Errorf("error = %q, want repeated key report", err.Error())
	}
	if cfg.Port != 0 {
		t.Errorf("Port = %d, want 0", cfg.Port)
	}

	// Repeating across invocations is also rejected.
	cfg2 := newKVGroupServer()
	fs2 := newKVGroupFlagSet(cfg2)
	if err := fs2.Parse([]string{"--group", "port=80"}); err != nil {
		t.Fatalf("first Parse: %v", err)
	}
	err = fs2.Parse([]string{"--group", "port=90"})
	if err == nil {
		t.Fatal("second Parse with repeated port = nil, want error")
	}
	if cfg2.Port != 80 {
		t.Errorf("Port = %d, want 80 (baseline preserved)", cfg2.Port)
	}
}

func TestKeyValueGroupCollectsAllErrors(t *testing.T) {
	cfg := newKVGroupServer()
	fs := newKVGroupFlagSet(cfg)

	err := fs.Parse([]string{"--group", "port=bad,unknown=x,tag=a,port=90"})
	if err == nil {
		t.Fatal("Parse = nil, want error")
	}
	msg := err.Error()
	for _, want := range []string{"unknown key", "may not be repeated", "invalid value"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not contain %q", msg, want)
		}
	}
	// Multi-value tag still published? No: whole batch fails.
	if len(cfg.Tags) != 0 {
		t.Errorf("Tags = %v, want empty (batch failed)", cfg.Tags)
	}
}

func TestKeyValueGroupEscapedSeparator(t *testing.T) {
	cfg := newKVGroupServer()
	fs := newKVGroupFlagSet(cfg)

	// Escaped comma must not split the value.
	if err := fs.Parse([]string{"--group", "tag=a" + kvBS + ",b"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cfg.Tags) != 1 || cfg.Tags[0] != "a,b" {
		t.Errorf("Tags = %v, want [a,b]", cfg.Tags)
	}

	// Escaped equals sign must not be treated as the key=value separator.
	cfg2 := newKVGroupServer()
	fs2 := newKVGroupFlagSet(cfg2)
	if err := fs2.Parse([]string{"--group", "tag=a" + kvBS + "=b"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cfg2.Tags) != 1 || cfg2.Tags[0] != "a=b" {
		t.Errorf("Tags = %v, want [a=b]", cfg2.Tags)
	}
}

func TestKeyValueGroupDefaults(t *testing.T) {
	cfg := &kvGroupServer{Port: 8080, Tags: []string{"default"}}
	fs := newKVGroupFlagSet(cfg)

	// No args: defaults preserved.
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse(nil): %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if len(cfg.Tags) != 1 || cfg.Tags[0] != "default" {
		t.Errorf("Tags = %v, want [default]", cfg.Tags)
	}

	// First set replaces defaults for multi-value fields.
	if err := fs.Parse([]string{"--group", "tag=a"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cfg.Tags) != 1 || cfg.Tags[0] != "a" {
		t.Errorf("Tags = %v, want [a] (defaults replaced)", cfg.Tags)
	}
}

func TestKeyValueGroupOtherFlagsUnaffected(t *testing.T) {
	cfg := newKVGroupServer()
	fs := newKVGroupFlagSet(cfg)
	other := fs.String("other", "def", "another flag")

	// A failing group parse must not change other flags.
	err := fs.Parse([]string{"--other", "xyz", "--group", "port=bad"})
	if err == nil {
		t.Fatal("Parse = nil, want error")
	}
	if *other != "xyz" {
		t.Errorf("other = %q, want xyz", *other)
	}
	if cfg.Port != 0 {
		t.Errorf("Port = %d, want 0", cfg.Port)
	}
}

func TestKeyValueGroupTypedFields(t *testing.T) {
	var (
		s    string
		i    int
		i64  int64
		u    uint
		u64  uint64
		b    bool
		f    float64
		d    time.Duration
		tags []string
	)
	g := NewKeyValueGroup().
		StringField("s", &s).
		IntField("i", &i).
		Int64Field("i64", &i64).
		UintField("u", &u).
		Uint64Field("u64", &u64).
		BoolField("b", &b).
		Float64Field("f", &f).
		DurationField("d", &d).
		StringSliceField("tags", &tags)
	fs := NewFlagSet("typed", ContinueOnError)
	fs.Var(g, "group", "")

	arg := "s=hello,i=42,i64=-7,u=3,u64=9,b=true,f=3.14,d=5s,tags=x,tags=y"
	if err := fs.Parse([]string{"--group", arg}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s != "hello" {
		t.Errorf("s = %q, want hello", s)
	}
	if i != 42 {
		t.Errorf("i = %d, want 42", i)
	}
	if i64 != -7 {
		t.Errorf("i64 = %d, want -7", i64)
	}
	if u != 3 {
		t.Errorf("u = %d, want 3", u)
	}
	if u64 != 9 {
		t.Errorf("u64 = %d, want 9", u64)
	}
	if !b {
		t.Errorf("b = false, want true")
	}
	if f != 3.14 {
		t.Errorf("f = %v, want 3.14", f)
	}
	if d != 5*time.Second {
		t.Errorf("d = %v, want 5s", d)
	}
	if len(tags) != 2 || tags[0] != "x" || tags[1] != "y" {
		t.Errorf("tags = %v, want [x y]", tags)
	}
}

func TestKeyValueGroupString(t *testing.T) {
	cfg := newKVGroupServer()
	fs := newKVGroupFlagSet(cfg)
	if err := fs.Parse([]string{"--group", "port=80,tag=a,tag=b"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	g := fs.Lookup("group").Value.(*KeyValueGroup)
	got := g.String()
	want := "[port=80,tag=a,tag=b]"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}

	// Values containing separators round-trip with escapes.
	cfg2 := newKVGroupServer()
	fs2 := newKVGroupFlagSet(cfg2)
	if err := fs2.Parse([]string{"--group", "tag=a" + kvBS + ",b"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	g2 := fs2.Lookup("group").Value.(*KeyValueGroup)
	got2 := g2.String()
	want2 := "[port=0,tag=a" + kvBS + ",b]"
	if got2 != want2 {
		t.Errorf("String() = %q, want %q", got2, want2)
	}
}

func TestKeyValueGroupIsValue(t *testing.T) {
	var _ Value = NewKeyValueGroup()
	g := NewKeyValueGroup()
	if g.Type() != "keyValueGroup" {
		t.Errorf("Type() = %q, want keyValueGroup", g.Type())
	}
	if g.changed {
		t.Error("changed = true for empty group, want false")
	}
}

func TestKeyValueGroupErrorIsError(t *testing.T) {
	cfg := newKVGroupServer()
	fs := newKVGroupFlagSet(cfg)
	err := fs.Parse([]string{"--group", "port=bad"})
	if err == nil {
		t.Fatal("Parse = nil, want error")
	}
	var kve *KeyValueGroupError
	if !errors.As(err, &kve) {
		t.Errorf("error %T is not *KeyValueGroupError", err)
	}
	if len(kve.Problems) != 1 {
		t.Errorf("Problems = %d, want 1", len(kve.Problems))
	}
}
