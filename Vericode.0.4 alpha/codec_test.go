package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── CSV ─────────────────────────────────────────────────────────────────────

func TestCSVParse(t *testing.T) {
	out := mustRun(t, `
let rows = csv_parse("name,age\nAda,36\n\"Bob, Jr\",5\n");
print len(rows);
print rows[2][0];
print rows[0][1];
print rows[1][1];
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"3", "Bob, Jr", "age", "36"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestCSVWriteForms(t *testing.T) {
	out := mustRun(t, `
print csv_write([["a", "b"], ["1", "x,y"]]);
print csv_write([{"b": 2, "a": 1}]);
let t = table([{"name": "Ada", "age": 36}], ["name", "age"]);
print csv_write(t);
`)
	if !strings.Contains(out, "a,b\n1,\"x,y\"\n") {
		t.Errorf("array-of-arrays output missing/wrong:\n%s", out)
	}
	if !strings.Contains(out, "a,b\n1,2\n") {
		t.Errorf("array-of-hashes output missing/wrong:\n%s", out)
	}
	if !strings.Contains(out, "name,age\nAda,36\n") {
		t.Errorf("table output missing/wrong:\n%s", out)
	}
}

func TestCSVTable(t *testing.T) {
	out := mustRun(t, `
let t = csv_table("name,score\nAda,95\nBob,88\n");
print len(t);
print columns(t);
print t[0]["score"];
print t["name"];
print t;
`)
	want := "2\n[name, score]\n95\n[Ada, Bob]\nname  score\n----  -----\nAda   95\nBob   88"
	if strings.TrimSpace(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestCSVTablePaddingAndErrors(t *testing.T) {
	out := mustRun(t, `
let t = csv_table("a,b\n1\n");
print t[0]["b"] == "";
print len(t[0]["b"]);
`)
	if strings.TrimSpace(out) != "true\n0" {
		t.Errorf("short row should pad with empty strings, got %q", out)
	}

	cases := []struct {
		name string
		src  string
		want string
	}{
		{"long row", `print csv_table("a,b\n1,2,3\n");`, "csv_table() row 1 has 3 fields, header has 2"},
		{"duplicate header", `print csv_table("a,a\n1,2\n");`, "csv_table() duplicate column 'a'"},
		{"bad csv", `print csv_parse("\"unclosed");`, "csv_parse()"},
		{"csv_write type", `print csv_write(5);`, "csv_write() expects a table"},
		{"mixed rows", `print csv_write([[1], 2]);`, "csv_write() element 1 must be an array"},
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

// ─── JSON ────────────────────────────────────────────────────────────────────

func TestJSONParseTypes(t *testing.T) {
	out := mustRun(t, `
let v = json_parse("{\"name\": \"Ada\", \"age\": 36, \"pi\": 3.5, \"tags\": [\"x\", \"y\"], \"ok\": true, \"none\": null}");
print v["name"];
print type(v["age"]);
print type(v["pi"]);
print v["tags"][1];
print v["ok"];
print type(v["none"]);
print v["age"] + 1;
`)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"Ada", "INTEGER", "FLOAT", "y", "true", "NULL", "37"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestJSONWrite(t *testing.T) {
	out := mustRun(t, `
let v = json_parse("{\"b\": 2, \"a\": [1, true, null]}");
print json_write(v);
print json_write([1, 2.5]);
let obj = {"name": "Ada"};
print json_write(obj, 2);
`)
	if !strings.Contains(out, `{"a":[1,true,null],"b":2}`) {
		t.Errorf("compact output wrong:\n%s", out)
	}
	if !strings.Contains(out, "[1,2.5]") {
		t.Errorf("array output wrong:\n%s", out)
	}
	if !strings.Contains(out, "{\n  \"name\": \"Ada\"\n}") {
		t.Errorf("indented output wrong:\n%s", out)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	out := mustRun(t, `
let original = {"n": [1, 2, 3], "ok": true, "label": "x"};
let copy = json_parse(json_write(original));
print copy["n"][2];
print copy["ok"];
print copy["label"];
`)
	if strings.TrimSpace(out) != "3\ntrue\nx" {
		t.Fatalf("round trip failed:\n%s", out)
	}
}

func TestJSONErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"empty", `print json_parse("");`, "json_parse() empty input"},
		{"invalid", `print json_parse("{");`, "json_parse()"},
		{"trailing", `print json_parse("1 2");`, "unexpected data after JSON value"},
		{"function value", `print json_write(func(x) { return x; });`, "cannot encode CLOSURE to JSON"},
		{"negative indent", `print json_write({"a": 1}, -1);`, "json_write() indent cannot be negative"},
		{"non-string key", `let h = {}; h[1] = "x"; print json_write(h);`, "hash keys must be strings, got INTEGER"},
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

func TestJSONWriteRejectsCycles(t *testing.T) {
	_, err := runSource(t, `
let a = [0];
let b = [a];
a[0] = b;
print json_write(a);
`)
	if err == nil {
		t.Fatal("expected a cycle error")
	}
	if !strings.Contains(err.Error(), "cyclic value") {
		t.Fatalf("error = %v, want cyclic value message", err)
	}
}

// ─── File I/O ────────────────────────────────────────────────────────────────

func runSourceWithIO(t testing.TB, input string, allowIO bool) (string, error) {
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
	vm.SetAllowIO(allowIO)
	if err := vm.Run(); err != nil {
		return "", err
	}
	return vm.Output(), nil
}

func TestFileIOWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.ToSlash(filepath.Join(dir, "data.txt"))
	src := fmt.Sprintf(`
write_file(%q, "hello file");
print read_file(%q);
`, path, path)
	out, err := runSourceWithIO(t, src, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "hello file" {
		t.Fatalf("got %q want hello file", out)
	}
	data, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello file" {
		t.Fatalf("file content = %q", data)
	}
}

func TestFileIODisabledByDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.ToSlash(filepath.Join(dir, "data.txt"))
	cases := []struct {
		name string
		src  string
	}{
		{"write", fmt.Sprintf(`write_file(%q, "x");`, path)},
		{"read", fmt.Sprintf(`print read_file(%q);`, path)},
		{"spawn inherits", fmt.Sprintf(`let work = func() { return read_file(%q); }; let task = spawn work(); print await task;`, path)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runSource(t, tc.src)
			if err == nil {
				t.Fatal("expected file access to be disabled")
			}
			if !strings.Contains(err.Error(), "disabled in the web IDE") {
				t.Fatalf("error = %v, want web IDE disabled message", err)
			}
		})
	}
}

func TestFileIOInheritedBySpawn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.ToSlash(filepath.Join(dir, "child.txt"))
	src := fmt.Sprintf(`
write_file(%q, "from parent");
let work = func() {
    return read_file(%q);
};
let task = spawn work();
print await task;
`, path, path)
	out, err := runSourceWithIO(t, src, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "from parent" {
		t.Fatalf("got %q want from parent", out)
	}
}

func TestFileIOErrors(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.ToSlash(filepath.Join(dir, "missing.txt"))
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"missing file", fmt.Sprintf(`print read_file(%q);`, missing), "read_file() cannot read"},
		{"path type", `print read_file(5);`, "read_file() path must be a string"},
		{"text type", `write_file("x.txt", 5);`, "write_file() text must be a string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runSourceWithIO(t, tc.src, true)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}
