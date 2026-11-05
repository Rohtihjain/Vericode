package main

import (
	"io"
	"strings"
	"testing"
	"time"
)

// runScriptConfig compiles and runs src with an explicit CLI configuration,
// returning the last popped value, captured output, and any runtime error.
func runScriptConfig(t *testing.T, src string, args []string, allowIO bool, stdin io.Reader) (Object, string, error) {
	t.Helper()
	l := NewLexer(src)
	p := NewParser(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, "", fmtError(joinErrors(errs))
	}
	c := NewCompilerWithSource("<input>", src)
	if err := c.Compile(program); err != nil {
		return nil, "", err
	}
	vm := NewVM(c.Bytecode())
	vm.SetAllowIO(allowIO)
	vm.SetArgs(args)
	if stdin != nil {
		vm.SetInput(stdin)
	}
	if err := vm.Run(); err != nil {
		return nil, "", err
	}
	return vm.LastPoppedStackElem(), vm.Output(), nil
}

func TestArgsExposedToScript(t *testing.T) {
	_, out, err := runScriptConfig(t, `
print len(args());
loop a in args() {
  print a;
}
`, []string{"hello", "world"}, true, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "2\nhello\nworld" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestArgsEmptyByDefault(t *testing.T) {
	_, out, err := runScriptConfig(t, `print len(args());`, nil, false, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "0" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestArgsVisibleInsideTask(t *testing.T) {
	_, out, err := runScriptConfig(t, `
func get() { return args(); }
let t = spawn get();
print join(await t, "-");
`, []string{"a", "b"}, true, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "a-b" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestEnvReadsHostValue(t *testing.T) {
	t.Setenv("VERI_TEST_GREETING", "hi there")
	_, out, err := runScriptConfig(t, `print env("VERI_TEST_GREETING");`, nil, true, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "hi there" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestEnvMissingIsNull(t *testing.T) {
	_, out, err := runScriptConfig(t, `print env("VERI_TEST_DEFINITELY_MISSING");`, nil, true, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "null" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestEnvGatedInWebIDE(t *testing.T) {
	_, _, err := runScriptConfig(t, `print env("PATH");`, nil, false, nil)
	if err == nil {
		t.Fatal("expected env() to be disabled without file I/O access")
	}
	if !strings.Contains(err.Error(), "env() is disabled in the web IDE") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNowReturnsMillis(t *testing.T) {
	before := time.Now().UnixMilli()
	val, _, err := runScriptConfig(t, `now();`, nil, false, nil)
	after := time.Now().UnixMilli()
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	n, ok := val.(*Integer)
	if !ok {
		t.Fatalf("expected integer, got %T", val)
	}
	if n.Value < before || n.Value > after {
		t.Fatalf("now() = %d, not between %d and %d", n.Value, before, after)
	}
}

func TestRandomFloatInUnitInterval(t *testing.T) {
	val, _, err := runScriptConfig(t, `random();`, nil, false, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	f, ok := val.(*Float)
	if !ok {
		t.Fatalf("expected float, got %T", val)
	}
	if f.Value < 0 || f.Value >= 1 {
		t.Fatalf("random() = %v, outside [0, 1)", f.Value)
	}
}

func TestRandomIntBound(t *testing.T) {
	val, _, err := runScriptConfig(t, `random(5);`, nil, false, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	n, ok := val.(*Integer)
	if !ok {
		t.Fatalf("expected integer, got %T", val)
	}
	if n.Value < 0 || n.Value >= 5 {
		t.Fatalf("random(5) = %d, outside [0, 5)", n.Value)
	}
}

func TestRandomTwoBoundsStayInRange(t *testing.T) {
	_, out, err := runScriptConfig(t, `
let ok = true;
loop i in range(0, 200) {
  let r = random(10, 15);
  if r < 10 { ok = false; }
  if r >= 15 { ok = false; }
}
print ok;
`, nil, false, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "true" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRandomRejectsInvalidBounds(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{`random(0);`, "random() bound must be positive"},
		{`random(5, 5);`, "random() low bound must be less than high bound"},
		{`random("x");`, "random() bound must be an integer"},
	}
	for _, tc := range cases {
		_, _, err := runScriptConfig(t, tc.src, nil, false, nil)
		if err == nil {
			t.Fatalf("%s: expected error", tc.src)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: unexpected error: %v", tc.src, err)
		}
	}
}

func TestAskReadsSuccessiveLines(t *testing.T) {
	_, out, err := runScriptConfig(t, `
let a = ask;
let b = ask;
print a + "|" + b;
`, nil, false, strings.NewReader("one\ntwo"))
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "one|two" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestAskSharedAcrossTasks(t *testing.T) {
	_, out, err := runScriptConfig(t, `
func get() { return ask; }
let t1 = spawn get();
let t2 = spawn get();
let a = await t1;
let b = await t2;
print contains([a, b], "alpha") && contains([a, b], "beta");
`, nil, false, strings.NewReader("alpha\nbeta"))
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "true" {
		t.Fatalf("unexpected output: %q", out)
	}
}
