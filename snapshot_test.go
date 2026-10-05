// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pflag

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestSnapshotSliceDefaultCopied(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	def := []string{"alpha", "beta"}
	var p []string
	f.StringSliceVar(&p, "slice", def, "a slice")
	def[0] = "MUTATED"
	def = append(def, "gamma")
	snap, err := f.DefaultSnapshot("slice")
	if err != nil {
		t.Fatalf("DefaultSnapshot: %v", err)
	}
	v, err := snap.Value()
	if err != nil {
		t.Fatalf("snap.Value: %v", err)
	}
	got := v.(SliceValue).GetSlice()
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("snapshot = %v, want [alpha beta]", got)
	}
	if p[0] != "MUTATED" {
		t.Fatalf("live value = %v, want mutated slice", p)
	}
}

func TestSnapshotIntSliceDefaultCopied(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	def := []int{1, 2, 3}
	var p []int
	f.IntSliceVar(&p, "ints", def, "ints")
	def[0] = 999
	snap, err := f.DefaultSnapshot("ints")
	if err != nil {
		t.Fatalf("DefaultSnapshot: %v", err)
	}
	v, err := snap.Value()
	if err != nil {
		t.Fatalf("snap.Value: %v", err)
	}
	got := v.(SliceValue).GetSlice()
	if len(got) != 3 || got[0] != "1" || got[1] != "2" || got[2] != "3" {
		t.Fatalf("snapshot = %v, want [1 2 3]", got)
	}
}

func TestSnapshotMapDefaultCopied(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	def := map[string]string{"a": "1"}
	var p map[string]string
	f.StringToStringVar(&p, "map", def, "a map")
	def["a"] = "MUTATED"
	def["b"] = "2"
	snap, err := f.DefaultSnapshot("map")
	if err != nil {
		t.Fatalf("DefaultSnapshot: %v", err)
	}
	v, err := snap.Value()
	if err != nil {
		t.Fatalf("snap.Value: %v", err)
	}
	m := v.(*stringToStringValue)
	if (*m.value)["a"] != "1" {
		t.Fatalf("snapshot map[a] = %q, want 1", (*m.value)["a"])
	}
	if _, ok := (*m.value)["b"]; ok {
		t.Fatalf("snapshot map contains b, want absent")
	}
}

func TestSnapshotParseAndSetSources(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	var s string
	f.StringVar(&s, "str", "default", "a string")
	var i int
	f.IntVar(&i, "num", 0, "an int")
	var b bool
	f.BoolVar(&b, "flag", false, "a bool")
	if src, err := f.ChangeSource("str"); err != nil || src != SourceDefault {
		t.Fatalf("initial source = %v, %v; want SourceDefault", src, err)
	}
	if err := f.Parse([]string{"--str=parsed", "--flag"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if src, err := f.ChangeSource("str"); err != nil || src != SourceParse {
		t.Fatalf("str source = %v, %v; want SourceParse", src, err)
	}
	if src, err := f.ChangeSource("flag"); err != nil || src != SourceParse {
		t.Fatalf("flag source = %v, %v; want SourceParse", src, err)
	}
	if src, err := f.ChangeSource("num"); err != nil || src != SourceDefault {
		t.Fatalf("num source = %v, %v; want SourceDefault", src, err)
	}
	if err := f.Set("num", "42"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if src, err := f.ChangeSource("num"); err != nil || src != SourceSet {
		t.Fatalf("num source = %v, %v; want SourceSet", src, err)
	}
	if err := f.Set("str", "set"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if src, err := f.ChangeSource("str"); err != nil || src != SourceSet {
		t.Fatalf("str source = %v, %v; want SourceSet (most recent write)", src, err)
	}
}

func TestSnapshotQueryDoesNotChangeChanged(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	var s string
	f.StringVar(&s, "str", "default", "a string")
	before := f.Lookup("str").Changed
	if _, err := f.DefaultSnapshot("str"); err != nil {
		t.Fatalf("DefaultSnapshot: %v", err)
	}
	if _, err := f.SnapshotChanged("str"); err != nil {
		t.Fatalf("SnapshotChanged: %v", err)
	}
	if _, err := f.ChangeSource("str"); err != nil {
		t.Fatalf("ChangeSource: %v", err)
	}
	after := f.Lookup("str").Changed
	if before != after {
		t.Fatalf("Changed changed from %v to %v after snapshot queries", before, after)
	}
	if err := f.Set("str", "x"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !f.Lookup("str").Changed {
		t.Fatalf("Changed should be true after Set")
	}
	if _, err := f.SnapshotChanged("str"); err != nil {
		t.Fatalf("SnapshotChanged: %v", err)
	}
	if !f.Lookup("str").Changed {
		t.Fatalf("SnapshotChanged query cleared Changed")
	}
}

func TestSnapshotBoolNoOpt(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	var b bool
	f.BoolVar(&b, "verbose", false, "verbose")
	if f.Lookup("verbose").NoOptDefVal != "true" {
		t.Fatalf("NoOptDefVal = %q, want true", f.Lookup("verbose").NoOptDefVal)
	}
	if err := f.Parse([]string{"--verbose"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !b {
		t.Fatalf("verbose = false after --verbose, want true")
	}
	if src, err := f.ChangeSource("verbose"); err != nil || src != SourceParse {
		t.Fatalf("verbose source = %v, %v; want SourceParse", src, err)
	}
}

func TestSnapshotFlagOrdering(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	f.String("zebra", "", "")
	f.String("apple", "", "")
	f.String("mango", "", "")
	snaps := f.DefaultSnapshots()
	if len(snaps) != 3 {
		t.Fatalf("got %d snapshots, want 3", len(snaps))
	}
	want := []string{"zebra", "apple", "mango"}
	for i, s := range snaps {
		if s.Name() != want[i] {
			t.Fatalf("snapshot[%d] = %q, want %q (registration order)", i, s.Name(), want[i])
		}
	}
}

func TestSnapshotCustomValue(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	cv := &customSnapValue{data: []string{"one", "two"}}
	f.Var(cv, "custom", "a custom value")
	cv.data[0] = "MUTATED"
	snap, err := f.DefaultSnapshot("custom")
	if err != nil {
		t.Fatalf("DefaultSnapshot: %v", err)
	}
	v, err := snap.Value()
	if err != nil {
		t.Fatalf("snap.Value: %v", err)
	}
	got := v.(*customSnapValue)
	if len(got.data) != 2 || got.data[0] != "one" || got.data[1] != "two" {
		t.Fatalf("snapshot = %v, want [one two]", got.data)
	}
	if got == cv {
		t.Fatalf("snapshot shares pointer with live value; want independent copy")
	}
}

func TestSnapshotUnknownValueRejected(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	uv := &unknownValue{}
	f.Var(uv, "unknown", "an unknown value type")
	snap, err := f.DefaultSnapshot("unknown")
	if err != nil {
		t.Fatalf("DefaultSnapshot: %v", err)
	}
	if _, err := snap.Value(); err == nil {
		t.Fatalf("snap.Value() = nil error, want rejection for unknown type")
	}
	if _, err := f.SnapshotChanged("unknown"); err == nil {
		t.Fatalf("SnapshotChanged() = nil error, want rejection for unknown type")
	}
	if _, err := f.ChangeSource("unknown"); err == nil {
		t.Fatalf("ChangeSource() = nil error, want rejection for unknown type")
	}
}

func TestSnapshotAllBuiltinTypes(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	var s string
	f.StringVar(&s, "string", "s", "")
	var b bool
	f.BoolVar(&b, "bool", true, "")
	var i int
	f.IntVar(&i, "int", 7, "")
	var i8 int8
	f.Int8Var(&i8, "int8", 8, "")
	var i16 int16
	f.Int16Var(&i16, "int16", 9, "")
	var i32 int32
	f.Int32Var(&i32, "int32", 10, "")
	var i64 int64
	f.Int64Var(&i64, "int64", 11, "")
	var u uint
	f.UintVar(&u, "uint", 12, "")
	var u8 uint8
	f.Uint8Var(&u8, "uint8", 13, "")
	var u16 uint16
	f.Uint16Var(&u16, "uint16", 14, "")
	var u32 uint32
	f.Uint32Var(&u32, "uint32", 15, "")
	var u64 uint64
	f.Uint64Var(&u64, "uint64", 16, "")
	var f32 float32
	f.Float32Var(&f32, "float32", 1.5, "")
	var f64 float64
	f.Float64Var(&f64, "float64", 2.5, "")
	var d time.Duration
	f.DurationVar(&d, "duration", 3*time.Second, "")
	var c int
	f.CountVar(&c, "count", "")
	var ip net.IP
	f.IPVar(&ip, "ip", net.ParseIP("1.2.3.4"), "")
	var ipnet net.IPNet
	f.IPNetVar(&ipnet, "ipnet", net.IPNet{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)}, "")
	var bb []byte
	f.BytesHexVar(&bb, "bytes", []byte{0xDE, 0xAD}, "")
	var ss []string
	f.StringSliceVar(&ss, "strslice", []string{"a", "b"}, "")
	var sa []string
	f.StringArrayVar(&sa, "strarray", []string{"x", "y"}, "")
	var is []int
	f.IntSliceVar(&is, "intslice", []int{1, 2}, "")
	var us []uint
	f.UintSliceVar(&us, "uintslice", []uint{3, 4}, "")
	var bs []bool
	f.BoolSliceVar(&bs, "boolslice", []bool{true, false}, "")
	var ds []time.Duration
	f.DurationSliceVar(&ds, "durslice", []time.Duration{time.Second}, "")
	var fs32 []float32
	f.Float32SliceVar(&fs32, "f32slice", []float32{1.1}, "")
	var fs64 []float64
	f.Float64SliceVar(&fs64, "f64slice", []float64{2.2}, "")
	var i32s []int32
	f.Int32SliceVar(&i32s, "i32slice", []int32{5}, "")
	var i64s []int64
	f.Int64SliceVar(&i64s, "i64slice", []int64{6}, "")
	var ips []net.IP
	f.IPSliceVar(&ips, "ipslice", []net.IP{net.ParseIP("1.1.1.1")}, "")
	var ipnets []net.IPNet
	f.IPNetSliceVar(&ipnets, "ipnetslice", []net.IPNet{{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)}}, "")
	var sti map[string]int
	f.StringToIntVar(&sti, "strtoint", map[string]int{"a": 1}, "")
	var sti64 map[string]int64
	f.StringToInt64Var(&sti64, "strtoint64", map[string]int64{"b": 2}, "")
	var tt time.Time
	f.TimeVar(&tt, "time", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), []string{time.RFC3339}, "")
	names := []string{
		"string", "bool", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "duration", "count", "ip", "ipnet",
		"bytes", "strslice", "strarray", "intslice", "uintslice",
		"boolslice", "durslice", "f32slice", "f64slice", "i32slice",
		"i64slice", "ipslice", "ipnetslice", "strtoint", "strtoint64", "time",
	}
	for _, name := range names {
		snap, err := f.DefaultSnapshot(name)
		if err != nil {
			t.Errorf("%s: DefaultSnapshot: %v", name, err)
			continue
		}
		if _, err := snap.Value(); err != nil {
			t.Errorf("%s: snap.Value: %v", name, err)
		}
	}
}

func TestSnapshotIPSliceDeepCopy(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	def := []net.IP{net.ParseIP("1.1.1.1")}
	var p []net.IP
	f.IPSliceVar(&p, "ips", def, "")
	def[0][0] = 9
	def = append(def, net.ParseIP("2.2.2.2"))
	snap, err := f.DefaultSnapshot("ips")
	if err != nil {
		t.Fatalf("DefaultSnapshot: %v", err)
	}
	v, err := snap.Value()
	if err != nil {
		t.Fatalf("snap.Value: %v", err)
	}
	got := v.(SliceValue).GetSlice()
	if len(got) != 1 || got[0] != "1.1.1.1" {
		t.Fatalf("snapshot = %v, want [1.1.1.1]", got)
	}
}

func TestSnapshotIPNetDeepCopy(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	def := net.IPNet{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)}
	var p net.IPNet
	f.IPNetVar(&p, "ipnet", def, "")
	def.IP[0] = 9
	def.Mask[0] = 0
	snap, err := f.DefaultSnapshot("ipnet")
	if err != nil {
		t.Fatalf("DefaultSnapshot: %v", err)
	}
	v, err := snap.Value()
	if err != nil {
		t.Fatalf("snap.Value: %v", err)
	}
	got := v.(*ipNetValue)
	if !got.IP.Equal(net.ParseIP("10.0.0.0")) || got.Mask[0] != 255 {
		t.Fatalf("snapshot IP=%v Mask=%v, want 10 and 255", got.IP, got.Mask)
	}
}

func TestSnapshotBytesDeepCopy(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	def := []byte{0xDE, 0xAD}
	var p []byte
	f.BytesHexVar(&p, "bytes", def, "")
	def[0] = 0xFF
	snap, err := f.DefaultSnapshot("bytes")
	if err != nil {
		t.Fatalf("DefaultSnapshot: %v", err)
	}
	v, err := snap.Value()
	if err != nil {
		t.Fatalf("snap.Value: %v", err)
	}
	got := v.(*bytesHexValue)
	if (*got)[0] != 0xDE {
		t.Fatalf("snapshot[0] = %X, want DE", (*got)[0])
	}
}

func TestSnapshotTimeValue(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	t0 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var p time.Time
	f.TimeVar(&p, "time", t0, []string{time.RFC3339}, "")
	t0 = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	snap, err := f.DefaultSnapshot("time")
	if err != nil {
		t.Fatalf("DefaultSnapshot: %v", err)
	}
	v, err := snap.Value()
	if err != nil {
		t.Fatalf("snap.Value: %v", err)
	}
	got := v.(*timeValue)
	if !got.Time.Equal(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("snapshot time = %v, want 2020-01-01", got.Time)
	}
}

func TestSnapshotNotFound(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	if _, err := f.DefaultSnapshot("nope"); err == nil {
		t.Fatalf("DefaultSnapshot(nope) = nil error, want error")
	}
	if _, err := f.SnapshotChanged("nope"); err == nil {
		t.Fatalf("SnapshotChanged(nope) = nil error, want error")
	}
	if _, err := f.ChangeSource("nope"); err == nil {
		t.Fatalf("ChangeSource(nope) = nil error, want error")
	}
}

func TestSnapshotSourceString(t *testing.T) {
	if SourceDefault.String() != "default" {
		t.Fatalf("SourceDefault.String() = %q", SourceDefault.String())
	}
	if SourceParse.String() != "parse" {
		t.Fatalf("SourceParse.String() = %q", SourceParse.String())
	}
	if SourceSet.String() != "set" {
		t.Fatalf("SourceSet.String() = %q", SourceSet.String())
	}
}

func TestSnapshotCurrentValue(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	var s string
	f.StringVar(&s, "str", "default", "")
	snap, err := f.DefaultSnapshot("str")
	if err != nil {
		t.Fatalf("DefaultSnapshot: %v", err)
	}
	if got := snap.Current().String(); got != "default" {
		t.Fatalf("Current() = %q, want default", got)
	}
	if err := f.Set("str", "live"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := snap.Current().String(); got != "live" {
		t.Fatalf("Current() = %q, want live", got)
	}
}

type customSnapValue struct {
	data []string
}

func (c *customSnapValue) String() string {
	return strings.Join(c.data, ",")
}

func (c *customSnapValue) Set(s string) error {
	c.data = strings.Split(s, ",")
	return nil
}

func (c *customSnapValue) Type() string { return "custom" }

func (c *customSnapValue) SnapshotValue() Value {
	out := make([]string, len(c.data))
	copy(out, c.data)
	return &customSnapValue{data: out}
}

type unknownValue struct{}

func (u *unknownValue) String() string     { return "unknown" }
func (u *unknownValue) Set(s string) error { return nil }
func (u *unknownValue) Type() string       { return "unknown" }
