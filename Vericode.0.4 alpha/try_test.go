package main

import (
	"strings"
	"testing"
)

// runScript runs a source program and returns both its output and its error,
// so tests can assert on partially completed output.
func runScript(t *testing.T, input string) (string, error) {
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
	err := vm.Run()
	return vm.Output(), err
}

func TestTryCatchDivisionByZero(t *testing.T) {
	out := mustRun(t, `
try {
  print 1 / 0;
} catch e {
  print "caught: " + e["message"];
}
print "after";
`)
	if out != "caught: division by zero\nafter" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestTrySkipsCatchWhenNoError(t *testing.T) {
	out := mustRun(t, `
try {
  print "body";
} catch e {
  print "handler";
}
`)
	if out != "body" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestCaughtErrorFields(t *testing.T) {
	out := mustRun(t, `
try {
  let a = 1;
  throw "boom";
} catch e {
  print e["message"];
  print e["line"];
  print e["col"];
  print e["type"];
  print e["file"];
}
`)
	want := "boom\n4\n3\nRuntimeError\n<input>"
	if out != want {
		t.Fatalf("unexpected output: %q want %q", out, want)
	}
}

func TestThrowNonStringUsesInspect(t *testing.T) {
	out := mustRun(t, `
try {
  throw 42;
} catch e {
  print e["message"];
}
`)
	if out != "42" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestTryNestedRethrow(t *testing.T) {
	out := mustRun(t, `
try {
  try {
    throw "inner";
  } catch e {
    throw "outer: " + e["message"];
  }
} catch e {
  print e["message"];
}
`)
	if out != "outer: inner" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestErrorInCatchPropagatesToOuterTry(t *testing.T) {
	out := mustRun(t, `
try {
  try {
    throw "one";
  } catch e {
    1 / 0;
  }
} catch e {
  print e["message"];
}
`)
	if out != "division by zero" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestHandlerPoppedBeforeCatchBody(t *testing.T) {
	out, err := runScript(t, `
try {
  1 / 0;
} catch e {
  print "caught";
}
print 1 / 0;
`)
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("expected uncaught division error, got %v", err)
	}
	if out != "caught" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestUncaughtThrowIsLocated(t *testing.T) {
	out, err := runScript(t, `
print "before";
throw "kaboom";
`)
	if err == nil || !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("expected throw error, got %v", err)
	}
	if !strings.Contains(err.Error(), "<input>:3:1") {
		t.Fatalf("expected throw location in error:\n%s", err)
	}
	if out != "before" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestCatchErrorFromCalledFunction(t *testing.T) {
	out := mustRun(t, `
func inner() {
  return 10 / 0;
}
try {
  inner();
} catch e {
  print e["message"];
  print contains(e["trace"], "inner");
}
`)
	if out != "division by zero\ntrue" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestReturnInsideTryLeavesNoHandler(t *testing.T) {
	out, err := runScript(t, `
func f() {
  try {
    return 1;
  } catch e {
    return 2;
  }
}
print f();
print 1 / 0;
`)
	if out != "1" {
		t.Fatalf("unexpected output: %q", out)
	}
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("expected uncaught error after return, got %v", err)
	}
}

func TestBreakInsideTryLeavesNoHandler(t *testing.T) {
	out, err := runScript(t, `
loop 1 times {
  try {
    break;
  } catch e {
    print "caught";
  }
}
print 1 / 0;
`)
	if out != "" {
		t.Fatalf("unexpected output: %q", out)
	}
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("expected uncaught error after break, got %v", err)
	}
}

func TestContinueInsideTry(t *testing.T) {
	out := mustRun(t, `
let s = 0;
loop i in [1, 2, 3] {
  try {
    if i == 2 { continue; }
    s = s + i;
  } catch e {
    s = -1;
  }
}
print s;
`)
	if out != "4" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestTryInsideLoopCatchesEveryIteration(t *testing.T) {
	out := mustRun(t, `
let count = 0;
loop 3 times {
  try {
    throw "x";
  } catch e {
    count = count + 1;
  }
}
print count;
`)
	if out != "3" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestCatchAwaitTaskFailure(t *testing.T) {
	out := mustRun(t, `
func boom() {
  throw "task boom";
}
let t = spawn boom;
try {
  await t;
} catch e {
  print e["message"];
}
`)
	if out != "task failed: task boom" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestCatchParallelCallbackFailure(t *testing.T) {
	out := mustRun(t, `
try {
  pmap([1, 2, 3], func(x) {
    if x == 2 { throw "bad item"; }
    return x;
  }, 2);
} catch e {
  print e["message"];
}
`)
	if out != "pmap failed: bad item" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestTryInsideParallelCallback(t *testing.T) {
	out := mustRun(t, `
let out = pmap([1, 2, 3], func(x) {
  try {
    if x == 2 { throw "skip"; }
    return x;
  } catch e {
    return 0;
  }
}, 3);
print out;
`)
	if out != "[1, 0, 3]" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestDeadlockCannotBeCaught(t *testing.T) {
	out, err := runScript(t, `
try {
  recv(chan());
} catch e {
  print "swallowed";
}
`)
	if err == nil || !strings.Contains(err.Error(), "deadlock") {
		t.Fatalf("expected deadlock error, got %v", err)
	}
	if strings.Contains(out, "swallowed") {
		t.Fatalf("deadlock was swallowed by catch: %q", out)
	}
}

func TestCancellationCannotBeCaught(t *testing.T) {
	out, err := runScript(t, `
func worker(ch) {
  try {
    recv(ch);
  } catch e {
    print "swallowed";
  }
  return 1;
}
let ch = chan();
let t = spawn worker(ch);
throw "parent failed";
`)
	if err == nil || !strings.Contains(err.Error(), "parent failed") {
		t.Fatalf("expected parent failure, got %v", err)
	}
	if strings.Contains(out, "swallowed") {
		t.Fatalf("cancellation was swallowed by catch: %q", out)
	}
}

func TestTryWithoutCatchIsParseError(t *testing.T) {
	_, err := runScript(t, `
try { print 1; }
print 2;
`)
	if err == nil || !strings.Contains(err.Error(), "expected 'catch'") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestCatchWithoutNameIsParseError(t *testing.T) {
	_, err := runScript(t, `
try { print 1; } catch { print 2; }
`)
	if err == nil || !strings.Contains(err.Error(), "expected") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestTryInFunctionLocalScope(t *testing.T) {
	out := mustRun(t, `
func safeDiv(a, b) {
  try {
    return a / b;
  } catch e {
    return 0;
  }
}
print safeDiv(10, 2);
print safeDiv(10, 0);
`)
	if out != "5\n0" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestCatchUnwindsThroughNestedCall(t *testing.T) {
	out := mustRun(t, `
func works() {
  return "ok";
}
func risky() {
  return 1 / 0;
}
func safe() {
  try {
    return works() + risky();
  } catch e {
    return "caught in safe";
  }
}
print safe();
`)
	if out != "caught in safe" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestCatchRestoresStackAfterTemporaries(t *testing.T) {
	out := mustRun(t, `
try {
  let arr = [1, 2 / 0, 3];
} catch e {
  print "caught";
}
print [4, 5, 6];
print 1 + 2;
`)
	if out != "caught\n[4, 5, 6]\n3" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestContinueInWhileInsideTry(t *testing.T) {
	out := mustRun(t, `
let i = 0;
let total = 0;
loop while i < 3 {
  i += 1;
  try {
    if i == 2 { continue; }
    total = total + i;
  } catch e {
    total = -1;
  }
}
print total;
`)
	if out != "4" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestCatchInlinePoolCallbackFailure(t *testing.T) {
	out := mustRun(t, `
try {
  pmap([1, 2, 3], func(x) {
    if x == 3 { throw "inline bad"; }
    return x;
  }, 1);
} catch e {
  print e["message"];
}
`)
	if out != "pmap failed: inline bad" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRethrowCaughtError(t *testing.T) {
	out := mustRun(t, `
try {
  try {
    throw "original";
  } catch e {
    throw e["message"];
  }
} catch e {
  print "again: " + e["message"];
}
`)
	if out != "again: original" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestCatchInsideSpawnedTask(t *testing.T) {
	out := mustRun(t, `
func guarded() {
  try {
    return 1 / 0;
  } catch e {
    return e["message"];
  }
}
let t = spawn guarded();
print await t;
`)
	if out != "division by zero" {
		t.Fatalf("unexpected output: %q", out)
	}
}
