/*
 Copyright 2021 The GoPlus Authors (goplus.org)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package cl_test

import (
	"bytes"
	"os"
	"path"
	"testing"

	"github.com/goplus/gop/cl"
	"github.com/goplus/gop/parser"
	"github.com/goplus/gop/parser/parsertest"
	"github.com/goplus/gop/scanner"
	"github.com/goplus/gox"
)

func newTwoFileFS(dir string, fname, data string, fname2 string, data2 string) *parsertest.MemFS {
	return parsertest.NewMemFS(map[string][]string{
		dir: {fname, fname2},
	}, map[string]string{
		path.Join(dir, fname):  data,
		path.Join(dir, fname2): data2,
	})
}

func gopSpxTest(t *testing.T, gmx, gopcode, expected string) {
	cl.SetDisableRecover(true)
	defer cl.SetDisableRecover(false)

	fs := newTwoFileFS("/foo", "bar.spx", gopcode, "index.gmx", gmx)
	pkgs, err := parser.ParseFSDir(gblFset, fs, "/foo", nil, 0)
	if err != nil {
		scanner.PrintError(os.Stderr, err)
		t.Fatal("ParseFSDir:", err)
	}
	conf := *baseConf.Ensure()
	bar := pkgs["main"]
	pkg, err := cl.NewPackage("", bar, &conf)
	if err != nil {
		t.Fatal("NewPackage:", err)
	}
	var b bytes.Buffer
	err = gox.WriteTo(&b, pkg, false)
	if err != nil {
		t.Fatal("gox.WriteTo failed:", err)
	}
	result := b.String()
	if result != expected {
		t.Fatalf("\nResult:\n%s\nExpected:\n%s\n", result, expected)
	}
}

func TestSpxBasic(t *testing.T) {
	gopSpxTest(t, `
const (
	GopGamePkg = "github.com/goplus/gop/cl/internal/spx"
	GopClass = "Game"
	GopThis = "this"
)

func onInit() {
}
`, `
const (
	GopClass = "Kai"
)

func onMsg(msg string) {
}
`, `package main

func onInit() {
}

const GopClass = "Kai"

func onMsg(msg string) {
}
`)
}

func TestSpxBasic2(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestSpxBasic2: no error?")
		}
	}()
	gopSpxTest(t, `
import (
	"fmt"
)

const (
	Foo = 1
)

func onInit() {
	fmt.Println("Hi")
}
`, ``, ``)
}

func TestSpxBasic3(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestSpxBasic3: no error?")
		}
	}()
	gopSpxTest(t, `
func onInit() {
}
`, ``, ``)
}

func _TestSpxVar(t *testing.T) {
	gopSpxTest(t, `
const (
	GopGamePkg = "github.com/goplus/cl/internal/spx"
	GopClass = "Game"
)

var (
	Kai Kai
)

func onInit() {
	Kai.clone()
	broadcast("msg1")
}
`, `
const (
	GopClass = "Kai"
)

func onInit() {
	setCostume("kai-a")
	play("recordingWhere")
	say("Where do you come from?", 2)
	broadcast("msg2")
}
`, `
`)
}
