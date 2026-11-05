package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompileCodeAcceptsProgram(t *testing.T) {
	bc, errMsg := compileCode(`print 1 + 2;`, "demo.veri")
	if errMsg != "" {
		t.Fatalf("unexpected compile error: %s", errMsg)
	}
	if bc == nil || len(bc.Instructions) == 0 {
		t.Fatal("compileCode returned empty bytecode")
	}
}

func TestCompileCodePointsAtFilename(t *testing.T) {
	_, errMsg := compileCode("let x = ;", "demo.veri")
	if errMsg == "" {
		t.Fatal("expected a parse error")
	}
	if !strings.Contains(errMsg, "demo.veri") {
		t.Fatalf("diagnostic does not name the file:\n%s", errMsg)
	}
	if !strings.Contains(errMsg, "ParseError") {
		t.Fatalf("diagnostic does not name the error type:\n%s", errMsg)
	}
}

func TestRunCodeInline(t *testing.T) {
	out, errMsg := runCode(`print 2 + 3;`, "<eval>", true, nil)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if out != "5" {
		t.Fatalf("output = %q, want 5", out)
	}
}

func TestRunCodeLiveStreamsLines(t *testing.T) {
	var live bytes.Buffer
	out, errMsg := runCodeLive(`print "one"; print "two";`, "<eval>", true, nil, &live)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if live.String() != "one\ntwo\n" {
		t.Fatalf("streamed output = %q", live.String())
	}
	if out != "one\ntwo" {
		t.Fatalf("collected output = %q", out)
	}
}

func TestRunCodeLiveStreamsFromTasks(t *testing.T) {
	var live bytes.Buffer
	src := `
func work(n) {
  print "task " + str(n);
}
let a = spawn work(1);
await a;
print "main";
`
	if _, errMsg := runCodeLive(src, "<eval>", true, nil, &live); errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if live.String() != "task 1\nmain\n" {
		t.Fatalf("streamed output = %q", live.String())
	}
}

func TestREPLKeepsGlobalsAndEchoes(t *testing.T) {
	r := newREPL(nil)

	if out, err := r.Eval("let x = 2;"); err != nil || out != "" {
		t.Fatalf("defining x: out=%q err=%v", out, err)
	}

	out, err := r.Eval("x + 3")
	if err != nil {
		t.Fatalf("using x failed: %v", err)
	}
	if out != "5" {
		t.Fatalf("echo = %q, want 5", out)
	}

	if _, err := r.Eval("1 / 0"); err == nil {
		t.Fatal("expected division by zero")
	}

	// A failed line must not end the session.
	out, err = r.Eval("x")
	if err != nil {
		t.Fatalf("session did not survive the error: %v", err)
	}
	if out != "2" {
		t.Fatalf("x after error = %q, want 2", out)
	}
}

func TestREPLFunctionsPersist(t *testing.T) {
	r := newREPL(nil)
	if _, err := r.Eval(`func double(n) { return n * 2; }`); err != nil {
		t.Fatalf("defining function: %v", err)
	}
	out, err := r.Eval("double(21)")
	if err != nil {
		t.Fatalf("calling function: %v", err)
	}
	if out != "42" {
		t.Fatalf("echo = %q, want 42", out)
	}
}

func TestREPLRejectsImports(t *testing.T) {
	r := newREPL(nil)
	_, err := r.Eval(`import math from "math.veri";`)
	if err == nil {
		t.Fatal("expected imports to be rejected in the REPL")
	}
	if !strings.Contains(err.Error(), "not supported in the REPL") {
		t.Fatalf("error = %v", err)
	}
}

func TestREPLMultilineDefinition(t *testing.T) {
	var out, errOut bytes.Buffer
	input := "func add(a, b) {\n  return a + b;\n}\nadd(10, 5)\n:quit\n"
	runREPL(strings.NewReader(input), &out, &errOut, nil)
	if errOut.Len() != 0 {
		t.Fatalf("unexpected REPL errors: %s", errOut.String())
	}
	if !strings.Contains(out.String(), "15") {
		t.Fatalf("REPL output does not contain 15:\n%s", out.String())
	}
}

func TestREPLResetForgetsNames(t *testing.T) {
	r := newREPL(nil)
	if _, err := r.Eval("let x = 1;"); err != nil {
		t.Fatalf("defining x: %v", err)
	}
	r.reset()
	if _, err := r.Eval("x"); err == nil {
		t.Fatal("expected x to be undefined after reset")
	}
}
