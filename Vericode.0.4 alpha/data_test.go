package main

import (
	"strings"
	"testing"
)

// ─── Pipelines ───────────────────────────────────────────────────────────────

func TestPipelineComposition(t *testing.T) {
	out := mustRun(t, `
let nums = range(1, 11);
let total = nums
    |> filter(func(x) { return x % 2 == 0; })
    |> map(func(x) { return x * x; })
    |> sum;
print total;

let first = range(1, 7)
    |> filter(func(x) { return x % 2 == 1; })
    |> map(func(x) { return x * 10; })
    |> take(2);
print first;
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"220", "[10, 30]"}
	if len(lines) != len(want) {
		t.Fatalf("got %q, want %v", out, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestPipeIntoUserFunction(t *testing.T) {
	out := mustRun(t, `
func triple(xs) {
    return map(xs, func(x) { return x * 3; });
}
print [1, 2, 3] |> triple;
print len([1, 2, 3] |> triple);
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if lines[0] != "[3, 6, 9]" {
		t.Errorf("got %q want [3, 6, 9]", lines[0])
	}
	if lines[1] != "3" {
		t.Errorf("got %q want 3", lines[1])
	}
}

func TestUserBindingsShadowBuiltins(t *testing.T) {
	out := mustRun(t, `
let sum = 10;
print sum + 1;

let map = [5];
print map[0];

func take(x) {
    return x + 1;
}
print take(41);

let work = func(x) {
    return x * 2;
};
print work(4);
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"11", "5", "42", "8"}
	if len(lines) != len(want) {
		t.Fatalf("got %q, want %v", out, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

// ─── Batch transforms ────────────────────────────────────────────────────────

func TestTakeDropChunk(t *testing.T) {
	out := mustRun(t, `
print take([1, 2, 3, 4], 2);
print take([1, 2], 5);
print take([1, 2], 0);
print drop([1, 2, 3, 4], 2);
print drop([1, 2], 5);
print chunk([1, 2, 3, 4, 5], 2);
print chunk([], 3);
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{
		"[1, 2]",
		"[1, 2]",
		"[]",
		"[3, 4]",
		"[]",
		"[[1, 2], [3, 4], [5]]",
		"[]",
	}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestZipEnumerateFlatten(t *testing.T) {
	out := mustRun(t, `
print zip([1, 2, 3], ["a", "b"]);
print enumerate(["x", "y"]);
print flatten([[1, 2], [3], 4]);
print flatten([]);
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"[[1, a], [2, b]]", "[[0, x], [1, y]]", "[1, 2, 3, 4]", "[]"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestUniq(t *testing.T) {
	out := mustRun(t, `
print uniq([1, 2, 1, 3, 2]);
print uniq(["a", "A", "a"]);
print uniq([[1, 2], [1, 2], [2]]);
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"[1, 2, 3]", "[a, A]", "[[1, 2], [2]]"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestSumAvg(t *testing.T) {
	out := mustRun(t, `
print sum([1, 2, 3]);
print sum([]);
print sum([1, 2.5]);
print avg([2, 4]);
print avg([1.0]);
print type(sum([1, 2]));
print type(sum([1.0, 2]));
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"6", "0", "3.5", "3", "1", "INTEGER", "FLOAT"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestSortReverse(t *testing.T) {
	out := mustRun(t, `
print sort([3, 1.5, 2]);
print sort(["pear", "apple", "fig"]);
print sort_by(["ccc", "a", "bb"], func(s) { return len(s); });
print sort([true, false]);
print sort([3, null, 1]);
print reverse([1, 2, 3]);
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{
		"[1.5, 2, 3]",
		"[apple, fig, pear]",
		"[a, bb, ccc]",
		"[false, true]",
		"[1, 3, null]",
		"[3, 2, 1]",
	}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestAnyAll(t *testing.T) {
	out := mustRun(t, `
print any([0, 0, 1]);
print all([1, 2, 3]);
print any([1, 2, 3], func(x) { return x > 2; });
print all([2, 4, 6], func(x) { return x % 2 == 0; });
print any([]);
print all([]);
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"true", "true", "true", "true", "false", "true"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestGroupByPluck(t *testing.T) {
	out := mustRun(t, `
let groups = group_by(range(0, 6), func(x) { return x % 2; });
print len(groups[0]);
print len(groups[1]);
print groups[0];
print groups[1];
print pluck([{"n": 1}, {"n": 2}], "n");
print pluck([{"n": 1}, {}], "n");
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"3", "3", "[0, 2, 4]", "[1, 3, 5]", "[1, 2]", "[1, null]"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestMinMaxArrayAndContains(t *testing.T) {
	out := mustRun(t, `
print min([3, 1, 2]);
print max([3, 1, 2]);
print min(4, 2, 8);
print contains([1, 2, 3], 2);
print contains([1, 2, 3], 9);
print contains([[1, 2]], [1, 2]);
print contains("abc", "b");
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"1", "3", "2", "true", "false", "true", "true"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestTransformErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"take negative", `print take([1, 2], -1);`, "take() count cannot be negative"},
		{"chunk zero", `print chunk([1, 2], 0);`, "chunk() size must be at least 1"},
		{"sum non-number", `print sum(["a"]);`, "sum() element 0 must be a number"},
		{"avg empty", `print avg([]);`, "avg() of an empty array"},
		{"sort mixed", `print sort([1, "a"]);`, "sort() cannot compare"},
		{"group key", `print group_by([1], func(x) { return [x]; });`, "group_by() key for element 0 must be hashable"},
		{"pluck type", `print pluck([1], "n");`, "pluck() element 0 must be a hash"},
		{"zip type", `print zip([1], 2);`, "zip() second argument must be an array"},
		{"min empty", `print min([]);`, "min() of an empty array"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runSource(t, tc.src)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestTransformErrorHasCallSite(t *testing.T) {
	_, err := runSource(t, `print take([1, 2], -1);`)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "--> <input>:1:7") {
		t.Fatalf("error should point at take() call site, got:\n%v", err)
	}
}

func TestCyclicValuesDoNotHang(t *testing.T) {
	out := mustRun(t, `
let a = [0];
let b = [a];
a[0] = b;
print contains(a, b);
print len(uniq([a, b]));
`)
	if strings.TrimSpace(out) != "true\n1" {
		t.Fatalf("got %q want true/1", out)
	}
}

func TestTransformCallbackErrorPropagates(t *testing.T) {
	_, err := runSource(t, `
let bad = func(x) {
    return x / 0;
};
print sort_by([1, 2], bad);
`)
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "division by zero") {
		t.Fatalf("missing root cause:\n%s", msg)
	}
	if !strings.Contains(msg, "at bad") {
		t.Fatalf("missing callback frame:\n%s", msg)
	}
}
