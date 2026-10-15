package main

import (
	"sort"
	"strings"
	"testing"
)

func TestPMapBasic(t *testing.T) {
	out := mustRun(t, `
let nums = range(1, 11);
print pmap(nums, func(x) { return x * x; });
`)
	if strings.TrimSpace(out) != "[1, 4, 9, 16, 25, 36, 49, 64, 81, 100]" {
		t.Errorf("got %q", out)
	}
}

func TestPMapWorkerCounts(t *testing.T) {
	// Element-dependent values catch both dropped and reordered work, which
	// is the classic parallel-map bug. Every pool size must agree.
	const src = `
let nums = range(0, 40);
print pmap(nums, func(x) { return x * 3 + 1; }, WORKERS);
`
	want := "["
	for i := 0; i < 40; i++ {
		if i > 0 {
			want += ", "
		}
		want += itoa(i*3+1)
	}
	want += "]"

	for _, w := range []string{"1", "2", "3", "7", "64"} {
		out := mustRun(t, strings.Replace(src, "WORKERS", w, 1))
		if strings.TrimSpace(out) != want {
			t.Errorf("workers=%s: got %q", w, out)
		}
	}
	// Default pool size (no workers argument) must agree too.
	out := mustRun(t, strings.Replace(src, ", WORKERS", "", 1))
	if strings.TrimSpace(out) != want {
		t.Errorf("default workers: got %q", out)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func TestPMapEmptyAndSingle(t *testing.T) {
	out := mustRun(t, `
print pmap([], func(x) { return x; });
print pmap([7], func(x) { return x * 2; });
`)
	if strings.TrimSpace(out) != "[]\n[14]" {
		t.Errorf("got %q", out)
	}
}

func TestPFilterOrder(t *testing.T) {
	for _, w := range []string{"1", "2", "5"} {
		out := mustRun(t, `
let nums = range(1, 21);
print pfilter(nums, func(x) { return x % 3 == 0; }, `+w+`);
`)
		if strings.TrimSpace(out) != "[3, 6, 9, 12, 15, 18]" {
			t.Errorf("workers=%s: got %q", w, out)
		}
	}
}

func TestPFilterEmptyResult(t *testing.T) {
	out := mustRun(t, `
print pfilter([1, 3, 5], func(x) { return x > 100; }, 2);
`)
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("got %q", out)
	}
}

func TestPForRange(t *testing.T) {
	out := mustRun(t, `
print pfor(3, 9, func(i) { return i * i; });
print pfor(5, 5, func(i) { return i; });
print pfor(9, 3, func(i) { return i; });
print pfor(0, 4, func(i) { return i; });
`)
	want := "[9, 16, 25, 36, 49, 64]\n[]\n[]\n[0, 1, 2, 3]"
	if strings.TrimSpace(out) != want {
		t.Errorf("got %q", out)
	}
}

func TestPForStress(t *testing.T) {
	out := mustRun(t, `
let r = pfor(0, 500, func(i) { return i * 2; }, 8);
let ok = true;
loop i in range(0, 500) {
  if r[i] != i * 2 { ok = false; }
}
print ok;
print len(r);
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || lines[0] != "true" || lines[1] != "500" {
		t.Errorf("got %q", out)
	}
}

func TestPMapTypeErrors(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{`pmap(5, func(x) { return x; });`, "pmap() first argument must be an array"},
		{`pmap([1], 5);`, "pmap() second argument must be a function"},
		{`pmap([1], func(a, b) { return a; });`, "pmap() callback must take 1 argument, got 2"},
		{`pmap([1], func(x) { return x; }, 0);`, "workers must be at least 1"},
		{`pmap([1], func(x) { return x; }, "two");`, "workers must be an integer"},
		{`pmap([1]);`, "pmap() takes 2-3 arguments"},
		{`pmap([1], func(x) { return x; }, 2, 3);`, "pmap() takes 2-3 arguments"},
	}
	for _, c := range cases {
		_, err := runSource(t, c.src)
		if err == nil {
			t.Errorf("%s: expected error", c.src)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %q want %q", c.src, err.Error(), c.want)
		}
	}
}

func TestPForTypeErrors(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{`pfor([1, 2], 5, func(i) { return i; });`, "pfor() start and end must be integers; for arrays use pmap()"},
		{`pfor(0, "x", func(i) { return i; });`, "pfor() start and end must be integers"},
		{`pfor(0, 3, 5);`, "pfor() third argument must be a function"},
		{`pfor(0, 3, func(a, b) { return a; });`, "pfor() callback must take 1 argument, got 2"},
		{`pfor(0);`, "pfor() takes 3-4 arguments"},
	}
	for _, c := range cases {
		_, err := runSource(t, c.src)
		if err == nil {
			t.Errorf("%s: expected error", c.src)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %q want %q", c.src, err.Error(), c.want)
		}
	}
}

func TestPMapErrorPropagatesPool(t *testing.T) {
	_, err := runSource(t, `
let r = pmap([1, 2, 0, 4], func(x) { return 100 / x; }, 3);
print r;
`)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{
		"pmap failed: division by zero",
		"-- inside parallel worker", // headline carries the marker too
		"--- inside parallel worker ---",
		"division by zero",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q:\n%s", want, msg)
		}
	}
	// The headline must sit at the pmap call site (line 2), not in the worker.
	if !strings.Contains(msg, "--> <input>:2:9") {
		t.Errorf("missing call-site location:\n%s", msg)
	}
}

func TestPMapErrorPropagatesInline(t *testing.T) {
	_, err := runSource(t, `
let r = pmap([1, 2, 0, 4], func(x) { return 100 / x; }, 1);
print r;
`)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{
		"pmap failed: division by zero",
		"--- inside parallel worker ---",
		"--> <input>:2:9",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q:\n%s", want, msg)
		}
	}
}

func TestPFilterErrorPropagates(t *testing.T) {
	_, err := runSource(t, `
let r = pfilter([1, 0, 2], func(x) { return 100 / x > 10; }, 2);
print r;
`)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"pfilter failed: division by zero", "division by zero"} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q:\n%s", want, msg)
		}
	}
}

func TestPForErrorPropagates(t *testing.T) {
	_, err := runSource(t, `
let r = pfor(0, 10, func(i) { return 10 / (5 - i); }, 2);
print r;
`)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "pfor failed: division by zero") {
		t.Errorf("got %v", err)
	}
}

func TestPMapBuiltinCallback(t *testing.T) {
	out := mustRun(t, `
print pmap(["a", "b", "c"], upper, 2);
`)
	if strings.TrimSpace(out) != "[A, B, C]" {
		t.Errorf("got %q", out)
	}
}

func TestPMapIsolation(t *testing.T) {
	// Callbacks mutate their own snapshot of the globals; the parent copy is
	// untouched and results are unaffected.
	out := mustRun(t, `
let g = [0];
let r = pmap(range(0, 20), func(i) { g = push(g, i); return i * 2; }, 4);
print g;
print len(r);
print r[19];
`)
	want := "[0]\n20\n38"
	if strings.TrimSpace(out) != want {
		t.Errorf("got %q", out)
	}
}

func TestPMapClosureCapture(t *testing.T) {
	out := mustRun(t, `
let factor = 3;
print pmap([1, 2, 3], func(x) { return x * factor; }, 3);
`)
	if strings.TrimSpace(out) != "[3, 6, 9]" {
		t.Errorf("got %q", out)
	}
}

func TestPMapNestedParallel(t *testing.T) {
	// The outer callback itself runs pmap — nested pools with a closure over
	// the parameter.
	out := mustRun(t, `
let r = pmap([2, 3], func(n) {
  let inner = pmap(range(0, n), func(i) { return i; }, 2);
  return len(inner);
}, 2);
print r;
`)
	if strings.TrimSpace(out) != "[2, 3]" {
		t.Errorf("got %q", out)
	}
}

func TestCallbackImplicitReturnIsNull(t *testing.T) {
	// Callbacks without an explicit return must yield null — not the last
	// value that happened to be popped.
	out := mustRun(t, `
print map([1, 2], func(x) { let y = x * 10; });
print pmap([1, 2], func(x) { let y = x * 10; }, 2);
`)
	if strings.TrimSpace(out) != "[null, null]\n[null, null]" {
		t.Errorf("got %q", out)
	}
}

func TestPMapPrintFromWorkers(t *testing.T) {
	out := mustRun(t, `
pfor(0, 6, func(i) { print i; }, 3);
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 6 {
		t.Fatalf("expected 6 lines, got %q", out)
	}
	sort.Strings(lines)
	for i, want := range []string{"0", "1", "2", "3", "4", "5"} {
		if lines[i] != want {
			t.Errorf("line %d = %q, want %q (all lines: %q)", i, lines[i], want, lines)
		}
	}
}

func TestPMapChannelFromWorkers(t *testing.T) {
	out := mustRun(t, `
let seen = chan(100);
pfor(0, 50, func(i) { send(seen, i); }, 4);
let total = 0;
loop k in range(0, 50) { total = total + recv(seen); }
print total;
`)
	if strings.TrimSpace(out) != "1225" {
		t.Errorf("got %q", out)
	}
}

func TestPMapSpawnInsideCallback(t *testing.T) {
	// Two elements with two workers: the callback truly runs on a pool
	// worker, so the spawned task's failure surfaces through the pool.
	_, err := runSource(t, `
let r = pmap([1, 2], func(x) {
  let t = spawn func() { return 1 / 0; }();
  return x;
}, 2);
print r;
`)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"pmap failed", "unawaited task", "division by zero"} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q:\n%s", want, msg)
		}
	}
}

func TestParallelCancellationWakesBlockedPool(t *testing.T) {
	// A task whose pmap callback blocks forever on recv must be woken by the
	// cancellation cascade when a sibling task fails — the program must not
	// hang. If this regresses, the test hangs until the go test timeout.
	_, err := runSource(t, `
let never = chan(0);
let stuck = spawn func() {
  pmap([1, 2, 3], func(x) { return recv(never); }, 2);
}();
let boom = spawn func() { sleep(80); return 1 / 0; }();
print await boom;
`)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "division by zero") {
		t.Errorf("got %v", err)
	}
}

func TestWorkersBuiltin(t *testing.T) {
	out := mustRun(t, `print workers() > 0;`)
	if strings.TrimSpace(out) != "true" {
		t.Errorf("got %q", out)
	}
	_, err := runSource(t, `workers(1);`)
	if err == nil || !strings.Contains(err.Error(), "workers() takes 0 arguments") {
		t.Errorf("got %v", err)
	}
}

func TestSequentialMapUnaffected(t *testing.T) {
	// The CallFn result-slot fix must not change sequential map/filter/reduce.
	out := mustRun(t, `
print map([1, 2, 3], func(x) { return x + 10; });
print filter([1, 2, 3, 4], func(x) { return x % 2 == 0; });
print reduce([1, 2, 3, 4], func(a, b) { return a + b; }, 0);
`)
	want := "[11, 12, 13]\n[2, 4]\n10"
	if strings.TrimSpace(out) != want {
		t.Errorf("got %q", out)
	}
}
