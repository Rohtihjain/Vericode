package main

import (
	"strings"
	"testing"
	"time"
)

func compileAndRun(t testing.TB, input string) (Object, error) {
	t.Helper()
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors:\n  %s", joinErrors(errs))
	}
	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		return nil, err
	}
	vm := NewVM(c.Bytecode())
	if err := vm.Run(); err != nil {
		return nil, err
	}
	return vm.LastPoppedStackElem(), nil
}

func joinErrors(errs []string) string {
	out := ""
	for i, e := range errs {
		if i > 0 {
			out += "\n  "
		}
		out += e
	}
	return out
}

func TestVariables(t *testing.T) {
	input := `
let a = 10;
let b = 20;
let c = a + b;
c;
`
	result, err := compileAndRun(t, input)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := result.(*Integer)
	if !ok {
		t.Fatalf("expected Integer, got %T (%v)", result, result)
	}
	if got.Value != 30 {
		t.Fatalf("expected 30, got %d", got.Value)
	}
}

func TestIfElse(t *testing.T) {
	input := `
let x = 5;
let result = 0;
if x > 3 {
    let result = 1;
} else {
    let result = 2;
}
result;
`
	// result at global scope remains 0 because if/else use local scopes in functions
	// Here it stays 0 — we just check it runs without error
	_, err := compileAndRun(t, input)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFunction(t *testing.T) {
	input := `
let add = func(a, b) { return a + b; };
let result = add(3, 4);
result;
`
	result, err := compileAndRun(t, input)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := result.(*Integer)
	if !ok {
		t.Fatalf("expected Integer, got %T", result)
	}
	if got.Value != 7 {
		t.Fatalf("expected 7, got %d", got.Value)
	}
}

func TestArray(t *testing.T) {
	input := `
let nums = [1, 2, 3];
nums[1];
`
	result, err := compileAndRun(t, input)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := result.(*Integer)
	if !ok {
		t.Fatalf("expected Integer, got %T", result)
	}
	if got.Value != 2 {
		t.Fatalf("expected 2, got %d", got.Value)
	}
}

func TestBuiltinLen(t *testing.T) {
	input := `
let s = "hello";
len(s);
`
	result, err := compileAndRun(t, input)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := result.(*Integer)
	if !ok {
		t.Fatalf("expected Integer, got %T", result)
	}
	if got.Value != 5 {
		t.Fatalf("expected 5, got %d", got.Value)
	}
}

func TestLoopTimes(t *testing.T) {
	input := `
let x = 0;
loop 3 times {
    let x = x + 1;
}
x;
`
	_, err := compileAndRun(t, input)
	if err != nil {
		t.Fatal(err)
	}
}

func TestVariableReassignment(t *testing.T) {
	input := `
let x = 10;
x = 25;
let f = func() {
    let y = 1;
    y = y + 4;
    return y;
};
let z = f();
x + z;
`
	res, err := compileAndRun(t, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	intVal, ok := res.(*Integer)
	if !ok || intVal.Value != 30 {
		t.Fatalf("expected 30, got %v", res)
	}
}

func TestLogicalOperators(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true && true;", true},
		{"true && false;", false},
		{"false && true;", false},
		{"false && false;", false},
		{"true || true;", true},
		{"true || false;", true},
		{"false || true;", true},
		{"false || false;", false},
		{"!true;", false},
		{"!false;", true},
		{"!(10 > 20);", true},
		{"(5 > 2) && (10 > 3);", true},
	}

	for _, tt := range tests {
		res, err := compileAndRun(t, tt.input)
		if err != nil {
			t.Fatalf("input %q failed: %v", tt.input, err)
		}
		b, ok := res.(*BooleanObj)
		if !ok || b.Value != tt.expected {
			t.Fatalf("input %q expected %v, got %v", tt.input, tt.expected, res)
		}
	}
}

func TestPipelineOperator(t *testing.T) {
	input := `
let nums = [1, 2, 3];
let res = nums |> push(4) |> len;
res;
`
	res, err := compileAndRun(t, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	intVal, ok := res.(*Integer)
	if !ok || intVal.Value != 4 {
		t.Fatalf("expected 4, got %v", res)
	}

	strInput := `
let name = "hello" |> upper;
name;
`
	resStr, err := compileAndRun(t, strInput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	strVal, ok := resStr.(*String)
	if !ok || strVal.Value != "HELLO" {
		t.Fatalf("expected HELLO, got %v", resStr)
	}
}

func TestHashMaps(t *testing.T) {
	input := `
let user = {"name": "VeriCode", "version": 1, "active": true};
let n = user["name"];
let v = user["version"];
let k = keys(user);
let h = has(user, "name");
let missing = user["unknown"];
[n, str(v), str(len(k)), str(h), str(missing)];
`
	res, err := compileAndRun(t, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	arr, ok := res.(*Array)
	if !ok || len(arr.Elements) != 5 {
		t.Fatalf("expected 5-element array, got %v", res)
	}
	if arr.Elements[0].Inspect() != "VeriCode" {
		t.Errorf("expected 'VeriCode', got %v", arr.Elements[0])
	}
	if arr.Elements[1].Inspect() != "1" {
		t.Errorf("expected '1', got %v", arr.Elements[1])
	}
	if arr.Elements[2].Inspect() != "3" {
		t.Errorf("expected '3' keys, got %v", arr.Elements[2])
	}
	if arr.Elements[3].Inspect() != "true" {
		t.Errorf("expected 'true' for has, got %v", arr.Elements[3])
	}
	if arr.Elements[4].Inspect() != "null" {
		t.Errorf("expected 'null' for missing key, got %v", arr.Elements[4])
	}
}

func TestBreakAndContinue(t *testing.T) {
	// Test break in while loop
	inputWhile := `
let i = 0;
loop while i < 10 {
    i = i + 1;
    if i == 5 {
        break;
    }
}
i;
`
	res, err := compileAndRun(t, inputWhile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := res.(*Integer)
	if !ok || val.Value != 5 {
		t.Fatalf("expected 5 from break, got %v", res)
	}

	// Test continue in while loop
	inputContinue := `
let i = 0;
let sum = 0;
loop while i < 5 {
    i = i + 1;
    if i == 3 {
        continue;
    }
    sum = sum + i;
}
sum;
`
	res2, err := compileAndRun(t, inputContinue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val2, ok := res2.(*Integer)
	// 1 + 2 + 4 + 5 = 12 (skipping 3)
	if !ok || val2.Value != 12 {
		t.Fatalf("expected 12 from continue, got %v", res2)
	}
}

func TestAsk(t *testing.T) {
	input := `
let name = ask;
name;
`
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	c := NewCompiler()
	if err := c.Compile(program); err != nil {
		t.Fatal(err)
	}
	vm := NewVM(c.Bytecode())
	vm.SetInput(strings.NewReader("Antigravity\n"))
	if err := vm.Run(); err != nil {
		t.Fatal(err)
	}
	res := vm.LastPoppedStackElem()
	s, ok := res.(*String)
	if !ok || s.Value != "Antigravity" {
		t.Fatalf("expected 'Antigravity', got %v", res)
	}
}

func TestNegativeNumbers(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{`let x = -5; x;`, -5},
		{`let x = -10 + 3; x;`, -7},
		{`let f = func(n) { return -n; }; f(7);`, -7},
		{`let x = 10; let y = -x; y;`, -10},
	}
	for _, tt := range tests {
		res, err := compileAndRun(t, tt.input)
		if err != nil {
			t.Fatalf("input %q failed: %v", tt.input, err)
		}
		iv, ok := res.(*Integer)
		if !ok || iv.Value != tt.expected {
			t.Fatalf("input %q expected %d, got %v", tt.input, tt.expected, res)
		}
	}
}

func TestStringEscapes(t *testing.T) {
	input := `let s = "hello\nworld"; s;`
	res, err := compileAndRun(t, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sv, ok := res.(*String)
	if !ok || sv.Value != "hello\nworld" {
		t.Fatalf("expected 'hello\\nworld', got %v", res)
	}

	input2 := `let s = "tab\there"; s;`
	res2, err := compileAndRun(t, input2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sv2, ok := res2.(*String)
	if !ok || sv2.Value != "tab\there" {
		t.Fatalf("expected tab escape, got %v", res2)
	}
}

func TestElseIf(t *testing.T) {
	input := `
let score = 75;
let grade = "unknown";
if score >= 90 {
    grade = "A";
} else if score >= 80 {
    grade = "B";
} else if score >= 70 {
    grade = "C";
} else {
    grade = "F";
}
grade;
`
	res, err := compileAndRun(t, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sv, ok := res.(*String)
	if !ok || sv.Value != "C" {
		t.Fatalf("expected 'C', got %v", res)
	}
}

func TestRange(t *testing.T) {
	input := `let r = range(5); len(r);`
	res, err := compileAndRun(t, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	iv, ok := res.(*Integer)
	if !ok || iv.Value != 5 {
		t.Fatalf("expected 5, got %v", res)
	}

	input2 := `let r = range(2, 7); r[0];`
	res2, err := compileAndRun(t, input2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	iv2, ok := res2.(*Integer)
	if !ok || iv2.Value != 2 {
		t.Fatalf("expected 2, got %v", res2)
	}
}

func TestMapFilterReduce(t *testing.T) {
	// map: double each element
	mapInput := `
let nums = [1, 2, 3, 4];
let doubled = map(nums, func(x) { return x * 2; });
doubled[2];
`
	res, err := compileAndRun(t, mapInput)
	if err != nil {
		t.Fatalf("map failed: %v", err)
	}
	iv, ok := res.(*Integer)
	if !ok || iv.Value != 6 {
		t.Fatalf("map: expected 6 (doubled index 2), got %v", res)
	}

	// filter: keep evens
	filterInput := `
let nums = [1, 2, 3, 4, 5, 6];
let evens = filter(nums, func(x) { return x % 2 == 0; });
len(evens);
`
	// Note: % not implemented yet, use (x / 2) * 2 == x
	filterInput2 := `
let nums = [1, 2, 3, 4, 5, 6];
let big = filter(nums, func(x) { return x > 3; });
len(big);
`
	res2, err := compileAndRun(t, filterInput2)
	if err != nil {
		t.Fatalf("filter failed: %v", err)
	}
	iv2, ok := res2.(*Integer)
	if !ok || iv2.Value != 3 {
		t.Fatalf("filter: expected 3 elements > 3, got %v", res2)
	}

	// reduce: sum
	reduceInput := `
let nums = [1, 2, 3, 4, 5];
let sum = reduce(nums, func(acc, x) { return acc + x; }, 0);
sum;
`
	res3, err := compileAndRun(t, reduceInput)
	if err != nil {
		t.Fatalf("reduce failed: %v", err)
	}
	iv3, ok := res3.(*Integer)
	if !ok || iv3.Value != 15 {
		t.Fatalf("reduce: expected 15, got %v", res3)
	}

	_ = filterInput // silence unused variable
}

func BenchmarkVM(b *testing.B) {
	input := `
let a = 10;
let b = 20;
let c = a + b;
c;
`
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()
	c := NewCompiler()
	_ = c.Compile(program)
	bytecode := c.Bytecode()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vm := NewVM(bytecode)
		_ = vm.Run()
	}
}

// ─── Helpers for new features ─────────────────────────────────────────────────

func runSource(t testing.TB, input string) (string, error) {
	t.Helper()
	l := NewLexer(input)
	p := NewParser(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return "", fmtError(joinErrors(errs))
	}
	c := NewCompilerWithSource("<input>", input)
	if err := c.Compile(program); err != nil {
		return "", err
	}
	vm := NewVM(c.Bytecode())
	if err := vm.Run(); err != nil {
		return "", err
	}
	return vm.Output(), nil
}

type fmtError string

func (e fmtError) Error() string { return string(e) }

func mustRun(t *testing.T, input string) string {
	t.Helper()
	out, err := runSource(t, input)
	if err != nil {
		t.Fatalf("run failed:\n%s\ninput:\n%s", err, input)
	}
	return out
}

func TestStringInterpolation(t *testing.T) {
	out := mustRun(t, `
let name = "Ada";
print "Hello, ${name}!";
print "sum=${1 + 2}";
print "len=${len([1,2,3])}";
`)
	if !strings.Contains(out, "Hello, Ada!") {
		t.Errorf("expected Hello, Ada! in output, got %q", out)
	}
	if !strings.Contains(out, "sum=3") {
		t.Errorf("expected sum=3 in output, got %q", out)
	}
	if !strings.Contains(out, "len=3") {
		t.Errorf("expected len=3 in output, got %q", out)
	}
}

func TestFormatBuiltin(t *testing.T) {
	out := mustRun(t, `
print format("Hi {}, score={}", "Ada", 95);
print format("plain");
print format("{}-{}-{}", 1, 2, 3);
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected 3 lines, got %q", out)
	}
	if lines[0] != "Hi Ada, score=95" {
		t.Errorf("line0 = %q", lines[0])
	}
	if lines[1] != "plain" {
		t.Errorf("line1 = %q", lines[1])
	}
	if lines[2] != "1-2-3" {
		t.Errorf("line2 = %q", lines[2])
	}
}

func TestMathBuiltins(t *testing.T) {
	out := mustRun(t, `
print abs(-7);
print abs(-3.5);
print min(3, 1, 7);
print max(3, 1, 7);
print sqrt(16.0);
print pow(2, 10);
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"7", "3.5", "1", "7", "4", "1024"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines %v, want %v", len(lines), lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestStringBuiltins(t *testing.T) {
	out := mustRun(t, `
print split("a,b,c", ",");
print join(["x", "y"], "-");
print trim("  hi  ");
print contains("vericode", "code");
print contains("vericode", "xyz");
print replace("hello", "l", "L");
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"[a, b, c]", "x-y", "hi", "true", "false", "heLLo"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestDiagnosticUndefined(t *testing.T) {
	_, err := runSource(t, `print nope;`)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "undefined variable") {
		t.Errorf("missing 'undefined variable': %s", msg)
	}
	if !strings.Contains(msg, "print nope;") {
		t.Errorf("missing source line: %s", msg)
	}
	if !strings.Contains(msg, "^") {
		t.Errorf("missing caret: %s", msg)
	}
}

func TestDiagnosticDivZero(t *testing.T) {
	_, err := runSource(t, `print 1 / 0;`)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "division by zero") {
		t.Errorf("missing message: %s", msg)
	}
	if !strings.Contains(msg, "print 1 / 0;") {
		t.Errorf("missing source line: %s", msg)
	}
}

func TestDiagnosticBuiltinError(t *testing.T) {
	_, err := runSource(t, `print abs("x");`)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "abs()") {
		t.Errorf("missing abs message: %s", msg)
	}
	// Caret should point near abs, not only the string arg
	if !strings.Contains(msg, "print abs") {
		t.Errorf("missing source context: %s", msg)
	}
}

func TestDiagnosticParseIncompleteLet(t *testing.T) {
	_, err := runSource(t, `let x =`)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, "unexpected token ''") {
		t.Errorf("still has weak empty-token message: %s", msg)
	}
	if !strings.Contains(msg, "expected expression") && !strings.Contains(msg, "unexpected end of input") {
		t.Errorf("expected clearer parse message, got: %s", msg)
	}
}

func TestDiagnosticBadInterpolation(t *testing.T) {
	_, err := runSource(t, `print "bad ${";`)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "unclosed interpolation") {
		t.Errorf("missing interpolation error: %s", msg)
	}
}

func TestStackTraceNestedCalls(t *testing.T) {
	_, err := runSource(t, `
func inner(a, b) {
  return a / b;
}
func middle(a, b) {
  return inner(a, b);
}
func outer(a, b) {
  return middle(a, b);
}
print outer(1, 0);
`)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Call Stack:") {
		t.Fatalf("missing call stack: %s", msg)
	}
	for _, frame := range []string{"at inner", "at middle", "at outer", "at <main>"} {
		if !strings.Contains(msg, frame) {
			t.Errorf("missing frame %q: %s", frame, msg)
		}
	}
	if strings.Index(msg, "at inner") > strings.Index(msg, "at middle") {
		t.Errorf("innermost frame should come first: %s", msg)
	}
}

func TestBuiltinErrorHasLocation(t *testing.T) {
	_, err := runSource(t, `print len(5);`)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "len() does not support INTEGER") {
		t.Errorf("missing message: %s", msg)
	}
	if !strings.Contains(msg, "--> <input>:1:7") {
		t.Errorf("missing location: %s", msg)
	}
	if !strings.Contains(msg, "print len(5);") {
		t.Errorf("missing source line: %s", msg)
	}
}

func TestMapCallbackErrorPropagates(t *testing.T) {
	_, err := runSource(t, `
func half(x) { return x / 2; }
print map([1, "a"], half);
`)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "unsupported operation") {
		t.Errorf("missing callback error: %s", msg)
	}
	if !strings.Contains(msg, "at half") {
		t.Errorf("missing callback frame in stack: %s", msg)
	}
}

func TestAwaitErrorCausalChain(t *testing.T) {
	_, err := runSource(t, `
func work() {
  return 1 / 0;
}
let t = spawn work();
let r = await t;
`)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{
		"task failed: division by zero",
		"inside task 'work'",
		"spawned at <input>:5:9",
		"--> <input>:3:12",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q: %s", want, msg)
		}
	}
}

func TestUnawaitedTaskNamesSpawnSite(t *testing.T) {
	_, err := runSource(t, `
func work() {
  return 1 / 0;
}
let t = spawn work();
print "main done";
`)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{
		"unawaited task 'work'",
		"spawned at <input>:5:9",
		"division by zero",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q: %s", want, msg)
		}
	}
}

func TestInterpolationWithExistingFeatures(t *testing.T) {
	out := mustRun(t, `
let nums = [10, 20];
print "first=${first(nums)} last=${last(nums)}";
let n = 0;
loop while n < 2 {
  n += 1;
}
print "n=${n}";
`)
	if !strings.Contains(out, "first=10 last=20") {
		t.Errorf("got %q", out)
	}
	if !strings.Contains(out, "n=2") {
		t.Errorf("got %q", out)
	}
}

func TestScientificFloats(t *testing.T) {
	out := mustRun(t, `
print 1e3;
print 2.5e2;
print 1e-2;
print 1.5E+1;
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"1000", "250", "0.01", "15"}
	if len(lines) != len(want) {
		t.Fatalf("got %v want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestArraySlicing(t *testing.T) {
	out := mustRun(t, `
let a = [10, 20, 30, 40, 50];
print a[1:3];
print a[:2];
print a[3:];
print a[:];
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"[20, 30]", "[10, 20]", "[40, 50]", "[10, 20, 30, 40, 50]"}
	if len(lines) != len(want) {
		t.Fatalf("got %v want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestStringSlicing(t *testing.T) {
	out := mustRun(t, `
let s = "hello";
print s[1:4];
print s[:2];
print s[3:];
print s[:];
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"ell", "he", "lo", "hello"}
	if len(lines) != len(want) {
		t.Fatalf("got %v want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

// ─── Concurrency (0.2) ───────────────────────────────────────────────────────

func TestSpawnAwaitResult(t *testing.T) {
	out := mustRun(t, `
let t = spawn func(a, b) { return a + b; }(19, 23);
print await t;
let u = spawn func() { return "hi"; }();
print await u;
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %q", out)
	}
	if lines[0] != "42" {
		t.Errorf("line0 = %q, want 42", lines[0])
	}
	if lines[1] != "hi" {
		t.Errorf("line1 = %q, want hi", lines[1])
	}
}

func TestSpawnWrongArgs(t *testing.T) {
	_, err := runSource(t, `
let t = spawn func(a, b) { return a; }(1);
await t;
`)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "wrong number of arguments") {
		t.Errorf("got %v", err)
	}
}

func TestAwaitErrorPropagation(t *testing.T) {
	_, err := runSource(t, `
let t = spawn func() { return 1 / 0; }();
await t;
`)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "division by zero") {
		t.Errorf("got %v", err)
	}
}

func TestUnawaitedTaskErrorSurfaces(t *testing.T) {
	_, err := runSource(t, `
let t = spawn func() { return 1 / 0; }();
print "main done";
`)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "division by zero") {
		t.Errorf("got %v", err)
	}
}

func TestChannelsBuffered(t *testing.T) {
	out := mustRun(t, `
let ch = chan(2);
send(ch, 10);
send(ch, 20);
let a = recv(ch);
let b = recv(ch);
print a + b;
`)
	if strings.TrimSpace(out) != "30" {
		t.Fatalf("got %q", out)
	}
}

func TestChannelsUnbufferedRendezvous(t *testing.T) {
	out := mustRun(t, `
let ch = chan();
let t = spawn func() {
    send(ch, "ping");
    return 0;
}();
sleep(50);
let v = recv(ch);
print v;
await t;
`)
	if strings.TrimSpace(out) != "ping" {
		t.Fatalf("got %q", out)
	}
}

func TestChannelsAreSharedAcrossTasks(t *testing.T) {
	out := mustRun(t, `
let ch = chan();
let t1 = spawn func() { send(ch, 1); return 0; }();
let t2 = spawn func() { send(ch, 2); return 0; }();
let a = recv(ch);
let b = recv(ch);
await t1;
await t2;
print a + b;
`)
	if strings.TrimSpace(out) != "3" {
		t.Fatalf("got %q", out)
	}
}

func TestSleepBuiltin(t *testing.T) {
	out := mustRun(t, `
sleep(20);
print "awake";
`)
	if strings.TrimSpace(out) != "awake" {
		t.Fatalf("got %q", out)
	}
}

func TestSnapshotIsolation(t *testing.T) {
	out := mustRun(t, `
let xs = [1, 2, 3];
let n = 0;
let getn = func() { return n; };
let t = spawn func() {
    xs = push(xs, 99);
    n = 100;
    return len(xs);
}();
let childLen = await t;
print childLen;
print len(xs);
print n;
print getn();
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"4", "3", "0", "0"}
	if len(lines) != 4 {
		t.Fatalf("got %v", lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestTaskInspect(t *testing.T) {
	out := mustRun(t, `
let t = spawn func() { sleep(200); return 0; }();
print t;
await t;
print t;
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %v", lines)
	}
	if !strings.Contains(lines[0], "<task") || !strings.Contains(lines[0], "running") {
		t.Errorf("running inspect: %q", lines[0])
	}
	if !strings.Contains(lines[1], "done") {
		t.Errorf("done inspect: %q", lines[1])
	}
}

func TestCancellationOnMainError(t *testing.T) {
	start := time.Now()
	_, err := runSource(t, `
let t = spawn func() { sleep(60000); return 0; }();
let x = 1 / 0;
print x;
`)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "division by zero") {
		t.Errorf("got %v", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("cancellation took too long: %v", elapsed)
	}
}

func TestConcurrencyStress(t *testing.T) {
	// Main and a child hammer channels/globals; run with -race this validates
	// the shared output collector and snapshot isolation.
	out := mustRun(t, `
let ch = chan(64);
let t = spawn func() {
    let i = 0;
    loop while i < 100 {
        send(ch, i);
        i += 1;
    }
    return 0;
}();
let sum = 0;
let i = 0;
loop while i < 100 {
    sum += recv(ch);
    i += 1;
}
await t;
print sum;
`)
	if strings.TrimSpace(out) != "4950" {
		t.Fatalf("got %q", out)
	}
}

func TestNamedFunctions(t *testing.T) {
	out := mustRun(t, `
func add(a, b) {
  return a + b;
}
print add(10, 20);

func fact(n) {
  if n <= 1 { return 1; }
  return n * fact(n - 1);
}
print fact(6);
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("got %q", out)
	}
	if lines[0] != "30" {
		t.Errorf("add: got %q", lines[0])
	}
	if lines[1] != "720" {
		t.Errorf("fact: got %q", lines[1])
	}
}
