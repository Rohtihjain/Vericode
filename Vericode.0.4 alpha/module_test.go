package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeModule writes a source file under dir, creating parent directories.
func writeModule(t *testing.T, dir, name, src string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// runFileAt compiles and runs src as if it lived at path.
func runFileAt(t *testing.T, path, src string, allowIO bool) (string, error) {
	t.Helper()
	l := NewLexer(src)
	p := NewParser(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return "", fmtError(joinErrors(errs))
	}
	c := NewCompilerWithSource(path, src)
	if err := c.Compile(program); err != nil {
		return "", err
	}
	vm := NewVM(c.Bytecode())
	vm.SetAllowIO(allowIO)
	if err := vm.Run(); err != nil {
		return "", err
	}
	return vm.Output(), nil
}

func mustRunFile(t *testing.T, path, src string) string {
	t.Helper()
	out, err := runFileAt(t, path, src, true)
	if err != nil {
		t.Fatalf("run failed:\n%s\nsource:\n%s", err, src)
	}
	return out
}

func TestImportBasics(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "math.veri", `
let PI = 3.14;
func add(a, b) { return a + b; }
func twice(x) { return x * 2; }
`)
	mainPath := filepath.Join(dir, "main.veri")
	out := mustRunFile(t, mainPath, `
import math from "math.veri";
print math["PI"];
print math["add"](2, 3);
print math["twice"](5);
`)
	if out != "3.14\n5\n10" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestImportRelativeToModule(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "sub/util.veri", `
func shout(s) { return upper(s); }
`)
	writeModule(t, dir, "sub/lib.veri", `
import util from "util.veri";
func greet(name) { return util["shout"]("hi " + name); }
`)
	mainPath := filepath.Join(dir, "main.veri")
	out := mustRunFile(t, mainPath, `
import lib from "sub/lib.veri";
print lib["greet"]("ada");
`)
	if out != "HI ADA" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestImportRunsOnce(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "once.veri", `
print "loaded";
let x = 7;
`)
	mainPath := filepath.Join(dir, "main.veri")
	out := mustRunFile(t, mainPath, `
import a from "once.veri";
import b from "once.veri";
print a["x"];
print b["x"];
`)
	if out != "loaded\n7\n7" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestImportInsideFunction(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "value.veri", `
let n = 41;
`)
	mainPath := filepath.Join(dir, "main.veri")
	out := mustRunFile(t, mainPath, `
func get() {
    import v from "value.veri";
    return v["n"] + 1;
}
print get();
print get();
`)
	if out != "42\n42" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestImportModulePrintsShareOutput(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "noisy.veri", `print "from module";`)
	mainPath := filepath.Join(dir, "main.veri")
	out := mustRunFile(t, mainPath, `
print "before";
import noisy from "noisy.veri";
print "after";
`)
	if out != "before\nfrom module\nafter" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestImportMissingFile(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.veri")
	_, err := runFileAt(t, mainPath, `import gone from "nope.veri";`, true)
	if err == nil {
		t.Fatal("expected an import error")
	}
	for _, want := range []string{"import 'nope.veri' failed", "cannot read module"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing %q in error: %v", want, err)
		}
	}
}

func TestImportCircular(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "a.veri", `
import b from "b.veri";
let x = 1;
`)
	writeModule(t, dir, "b.veri", `
import a from "a.veri";
let y = 2;
`)
	mainPath := filepath.Join(dir, "main.veri")
	_, err := runFileAt(t, mainPath, `import a from "a.veri";`, true)
	if err == nil {
		t.Fatal("expected a circular-import error")
	}
	if !strings.Contains(err.Error(), "circular import") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImportRuntimeErrorPointsAtModule(t *testing.T) {
	dir := t.TempDir()
	modPath := writeModule(t, dir, "bad.veri", `
print 1 / 0;
`)
	mainPath := filepath.Join(dir, "main.veri")
	_, err := runFileAt(t, mainPath, `import bad from "bad.veri";`, true)
	if err == nil {
		t.Fatal("expected a runtime error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "import 'bad.veri' failed") {
		t.Fatalf("missing import context: %s", msg)
	}
	if !strings.Contains(msg, "division by zero") {
		t.Fatalf("missing cause: %s", msg)
	}
	if !strings.Contains(msg, filepath.Base(modPath)) {
		t.Fatalf("missing module filename in diagnostic: %s", msg)
	}
}

func TestImportErrorIsCatchable(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.veri")
	out, err := runFileAt(t, mainPath, `
try {
    import gone from "nope.veri";
} catch e {
    print "caught import failure";
}
`, true)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "caught import failure" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestImportGatedInWebIDE(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "lib.veri", `let x = 1;`)
	mainPath := filepath.Join(dir, "main.veri")
	_, err := runFileAt(t, mainPath, `import lib from "lib.veri";`, false)
	if err == nil {
		t.Fatal("expected imports to be disabled without file I/O access")
	}
	if !strings.Contains(err.Error(), "import is disabled in the web IDE") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImportNamespaceIsMutableAndPrivate(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "counter.veri", `let n = 1;`)
	mainPath := filepath.Join(dir, "main.veri")
	out := mustRunFile(t, mainPath, `
import c from "counter.veri";
c["n"] = c["n"] + 1;
print c["n"];
`)
	// The namespace is a plain hash: writing it does not disturb the module.
	if out != "2" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestImportInsideModuleFunctionResolvesToModuleDir(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "sub/helper.veri", `let x = 5;`)
	writeModule(t, dir, "sub/lib.veri", `
func get() {
    import h from "helper.veri";
    return h["x"];
}
`)
	mainPath := filepath.Join(dir, "main.veri")
	out := mustRunFile(t, mainPath, `
import lib from "sub/lib.veri";
print lib["get"]();
`)
	if out != "5" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestImportExportsTopLevelNames(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "loops.veri", `
let total = 0;
loop i in range(0, 4) {
    total += i;
}
`)
	mainPath := filepath.Join(dir, "main.veri")
	out := mustRunFile(t, mainPath, `
import loops from "loops.veri";
print loops["total"];
// Top-level loop variables are globals too, so both names are exported.
print len(keys(loops)) == 2;
print has(loops, "total") && has(loops, "i");
`)
	if out != "6\ntrue\ntrue" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestImportParseErrorPointsAtModule(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, dir, "broken.veri", "let x = ;\n")
	mainPath := filepath.Join(dir, "main.veri")
	_, err := runFileAt(t, mainPath, `import broken from "broken.veri";`, true)
	if err == nil {
		t.Fatal("expected a parse error from the module")
	}
	if !strings.Contains(err.Error(), "broken.veri") {
		t.Fatalf("error does not name the module:\n%s", err)
	}
	if !strings.Contains(err.Error(), "ParseError") {
		t.Fatalf("error does not name the diagnostic type:\n%s", err)
	}
}
