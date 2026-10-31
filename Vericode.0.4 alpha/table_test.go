package main

import (
	"strings"
	"testing"
)

func TestTableBasics(t *testing.T) {
	out := mustRun(t, `
let people = table([
    {"name": "Ada", "age": 36},
    {"name": "Bob", "age": 5}
], ["name", "age"]);
print len(people);
print columns(people);
print type(people);
print has(people, "age");
print has(people, "nope");
print people[0]["name"];
print people[1]["age"];
print people["name"];
print rows(people)[1]["name"];
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"2", "[name, age]", "TABLE", "true", "false", "Ada", "5", "[Ada, Bob]", "Bob"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestTableDefaultHeadersAreSorted(t *testing.T) {
	out := mustRun(t, `
let t = table([{"b": 1, "a": 2}]);
print columns(t);
print t;
`)
	want := "[a, b]\na  b\n-  -\n2  1"
	if strings.TrimSpace(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestTableInspectAlignment(t *testing.T) {
	out := mustRun(t, `
let people = table([
    {"name": "Ada", "age": 36},
    {"name": "Bob", "age": 5}
], ["name", "age"]);
print people;
`)
	want := "name  age\n----  ---\nAda   36\nBob   5"
	if strings.TrimSpace(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestTableExplicitHeadersPadMissing(t *testing.T) {
	out := mustRun(t, `
let t = table([{"a": 1}, {"b": 2}], ["a", "b"]);
print t[0]["b"];
print t[1]["a"];
print columns(t);
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"null", "null", "[a, b]"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestTableIndexAndMutation(t *testing.T) {
	out := mustRun(t, `
let t = table([{"n": 1}], ["n"]);
t[0]["n"] = 42;
print t[0]["n"];
print t["n"];
print t[5];
print t["missing"];
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"42", "[42]", "null", "null"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestTableWhereSortSelect(t *testing.T) {
	out := mustRun(t, `
let t = table([
    {"name": "Ada", "age": 36, "city": "London"},
    {"name": "Bob", "age": 5, "city": "Paris"},
    {"name": "Cy", "age": 41, "city": "London"}
], ["name", "age", "city"]);

let adults = t |> twhere(func(row) { return row["age"] >= 18; });
print len(adults);

let sorted = adults |> tsort("age", "desc");
print sorted[0]["name"];
print sorted[1]["name"];

let names = t |> tselect("name");
print columns(names);
print names["name"];
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"2", "Cy", "Ada", "[name]", "[Ada, Bob, Cy]"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestTableSortNullsLast(t *testing.T) {
	out := mustRun(t, `
let t = table([{"n": 3}, {"n": null}, {"n": 1}], ["n"]);
print t |> tsort("n") |> pluck("n");
print t |> tsort("n", "desc") |> pluck("n");
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"[1, 3, null]", "[3, 1, null]"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestTableGroupAndPluck(t *testing.T) {
	out := mustRun(t, `
let t = table([
    {"city": "London", "n": 1},
    {"city": "Paris", "n": 2},
    {"city": "London", "n": 3}
], ["city", "n"]);
let groups = tgroup(t, "city");
print len(keys(groups));
print len(groups["London"]);
print groups["London"]["n"];
print len(groups["Paris"]);
print pluck(t, "n");
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"2", "2", "[1, 3]", "1", "[1, 2, 3]"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestTableCsvAndJsonWrite(t *testing.T) {
	out := mustRun(t, `
let t = table([{"name": "Ada", "age": 36}], ["name", "age"]);
print csv_write(t);
print json_write(t);
`)
	if !strings.Contains(out, "name,age\nAda,36\n") {
		t.Errorf("csv_write output missing table rows, got %q", out)
	}
	if !strings.Contains(out, `[{"age":36,"name":"Ada"}]`) {
		t.Errorf("json_write output missing table rows, got %q", out)
	}
}

func TestTableConstructionErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"non-hash row", `print table([1]);`, "table() row 0 must be a hash, got INTEGER"},
		{"duplicate header", `print table([{"a": 1}], ["a", "a"]);`, "table() duplicate column 'a'"},
		{"unknown column", `print table([{"a": 1, "b": 2}], ["a"]);`, "table() row 0 has column 'b' not listed in headers"},
		{"select unknown", `print tselect(table([{"a": 1}]), "b");`, "tselect() unknown column 'b'"},
		{"sort unknown", `print tsort(table([{"a": 1}]), "b");`, "tsort() unknown column 'b'"},
		{"sort order", `print tsort(table([{"a": 1}]), "a", "up");`, `tsort() order must be "asc" or "desc"`},
		{"group unknown", `print tgroup(table([{"a": 1}]), "b");`, "tgroup() unknown column 'b'"},
		{"where not func", `print twhere(table([{"a": 1}]), 2);`, "twhere() second argument must be a function"},
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

func TestTableCallbackErrorPropagates(t *testing.T) {
	_, err := runSource(t, `
let t = table([{"n": 1}], ["n"]);
print t |> twhere(func(row) { return row["n"] / 0; });
`)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("missing root cause:\n%v", err)
	}
}

func TestTableSnapshotIsolation(t *testing.T) {
	out := mustRun(t, `
let t = table([{"n": 1}], ["n"]);
let work = func() {
    t[0]["n"] = 99;
    return t[0]["n"];
};
let task = spawn work();
print await task;
print t[0]["n"];
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if lines[0] != "99" {
		t.Errorf("child saw %q, want 99", lines[0])
	}
	if lines[1] != "1" {
		t.Errorf("parent table changed: %q, want 1", lines[1])
	}
}

func TestTableWithParallelRows(t *testing.T) {
	out := mustRun(t, `
let t = table([{"n": 1}, {"n": 2}, {"n": 3}], ["n"]);
print rows(t) |> pmap(func(row) { return row["n"] * 10; }, 2);
`)
	if strings.TrimSpace(out) != "[10, 20, 30]" {
		t.Fatalf("got %q want [10, 20, 30]", out)
	}
}
