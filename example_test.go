// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pflag_test

import (
	"fmt"

	"github.com/spf13/pflag"
)

func ExampleShorthandLookup() {
	name := "verbose"
	short := name[:1]

	pflag.BoolP(name, short, false, "verbose output")

	// len(short) must be == 1
	flag := pflag.ShorthandLookup(short)

	fmt.Println(flag.Name)
}

func ExampleFlagSet_ShorthandLookup() {
	name := "verbose"
	short := name[:1]

	fs := pflag.NewFlagSet("Example", pflag.ContinueOnError)
	fs.BoolP(name, short, false, "verbose output")

	// len(short) must be == 1
	flag := fs.ShorthandLookup(short)

	fmt.Println(flag.Name)
}

func ExampleFlagSet_StringToString() {
	args := []string{
		"--arg", "a=1,b=2",
		"--arg", "a=2",
		"--arg=d=4",
	}

	fs := pflag.NewFlagSet("Example", pflag.ContinueOnError)
	fs.StringToString("arg", make(map[string]string), "string-to-string arg accepting key=value pairs")

	if err := fs.Parse(args); err != nil {
		panic(err)
	}

	value, err := fs.GetStringToString("arg")
	if err != nil {
		panic(err)
	}

	fmt.Println(value)
	// Output: map[a:2 b:2 d:4]
}

func ExampleIntListVar() {
	fs := pflag.NewFlagSet("Example", pflag.ContinueOnError)
	var list []int
	fs.IntListVar(&list, "il", []int{1, 2}, "integer list supporting =replace, +append, -delete operations")

	// Repeated occurrences run in command-line order.
	if err := fs.Parse([]string{"--il=+3", "--il=-1", "--il=+2"}); err != nil {
		panic(err)
	}
	fmt.Println(list)

	// A failed operation leaves the list at its baseline.
	if err := fs.Parse([]string{"--il=+bad"}); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Println(list)
	// Output: [2 3 2]
	// error: invalid argument "+bad" for "--il" flag: strconv.Atoi: parsing "bad": invalid syntax
	// [2 3 2]
}

func ExampleIntListVar_negative() {
	fs := pflag.NewFlagSet("Example", pflag.ContinueOnError)
	var list []int
	fs.IntListVar(&list, "il", []int{1, -2}, "integer list supporting =replace, +append, -delete operations")

	// Negative integers are parenthesized so they are not confused with the
	// delete operator: +(-3) appends -3, while -3 deletes every 3.
	if err := fs.Parse([]string{"--il=+(-3)", "--il=-(-2)"}); err != nil {
		panic(err)
	}
	fmt.Println(list)
	// Output: [1 -3]
}
