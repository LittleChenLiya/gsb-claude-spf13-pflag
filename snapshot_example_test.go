// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pflag_test

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

// repeatValue is a custom pflag.Value that stores a list of strings. It
// implements pflag.ValueSnapshot so that pflag can record an independent
// copy of the default value at registration time.
type repeatValue struct {
	parts []string
}

func (r *repeatValue) String() string { return strings.Join(r.parts, ",") }
func (r *repeatValue) Set(s string) error {
	r.parts = strings.Split(s, ",")
	return nil
}
func (r *repeatValue) Type() string { return "repeat" }

// SnapshotValue implements pflag.ValueSnapshot. It returns a deep copy
// of the value so later mutations of the receiver do not affect the
// snapshot.
func (r *repeatValue) SnapshotValue() pflag.Value {
	out := make([]string, len(r.parts))
	copy(out, r.parts)
	return &repeatValue{parts: out}
}

// ExampleValueSnapshot demonstrates how to implement pflag.ValueSnapshot
// and how the default snapshot records the registration-time default.
func ExampleValueSnapshot() {
	fs := pflag.NewFlagSet("example", pflag.ContinueOnError)

	// Register a flag with a default value.
	def := &repeatValue{parts: []string{"a", "b"}}
	fs.Var(def, "repeat", "repeatable value")

	// Mutate the default after registration; the snapshot must keep the
	// original content.
	def.parts[0] = "MUTATED"

	snap, err := fs.DefaultSnapshot("repeat")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	v, err := snap.Value()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("snapshot default: %s\n", v.String())
	fmt.Printf("current value:    %s\n", snap.Current().String())

	// Output:
	// snapshot default: a,b
	// current value:    MUTATED,b
}

// ExampleFlagSet_DefaultSnapshot demonstrates how to query the default
// snapshot and the change source (Parse vs Set).
func ExampleFlagSet_DefaultSnapshot() {
	fs := pflag.NewFlagSet("example", pflag.ContinueOnError)
	var s string
	fs.StringVar(&s, "name", "default", "a name")

	// Before any change the source is default.
	snap, _ := fs.DefaultSnapshot("name")
	fmt.Printf("before parse: changed=%v source=%s\n", snap.Changed(), snap.Source())

	// Parse changes the value; the source is reported as parse.
	_ = fs.Parse([]string{"--name=parsed"})
	snap, _ = fs.DefaultSnapshot("name")
	fmt.Printf("after parse:  changed=%v source=%s\n", snap.Changed(), snap.Source())

	// A direct Set changes the value; the source is reported as set.
	_ = fs.Set("name", "set")
	snap, _ = fs.DefaultSnapshot("name")
	fmt.Printf("after set:    changed=%v source=%s\n", snap.Changed(), snap.Source())

	// Querying the snapshot does not clear the Changed flag.
	fmt.Printf("changed after query: %v\n", fs.Lookup("name").Changed)

	// Output:
	// before parse: changed=false source=default
	// after parse:  changed=true source=parse
	// after set:    changed=true source=set
	// changed after query: true
}

// ExampleFlagSet_DefaultSnapshots demonstrates that DefaultSnapshots
// returns flags in registration order, not sorted order.
func ExampleFlagSet_DefaultSnapshots() {
	fs := pflag.NewFlagSet("example", pflag.ContinueOnError)
	fs.String("zebra", "", "")
	fs.String("apple", "", "")
	fs.String("mango", "", "")

	for _, snap := range fs.DefaultSnapshots() {
		fmt.Println(snap.Name())
	}

	// Output:
	// zebra
	// apple
	// mango
}
