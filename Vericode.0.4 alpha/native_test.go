package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExecHelper is not a normal test: TestExecCapturesOutputAndExitCode runs
// the test binary again with this test selected, and it behaves like a small
// external command.
func TestExecHelper(t *testing.T) {
	if os.Getenv("VERI_EXEC_HELPER") != "1" {
		t.Skip("helper process for exec() tests")
	}
	fmt.Print("out-line")
	fmt.Fprint(os.Stderr, "err-line")
	os.Exit(3)
}

func TestExecCapturesOutputAndExitCode(t *testing.T) {
	t.Setenv("VERI_EXEC_HELPER", "1")
	src := `
let r = exec(args()[0], ["-test.run=TestExecHelper"]);
print r["code"];
print r["stdout"];
print r["stderr"];
`
	_, out, err := runScriptConfig(t, src, []string{os.Args[0]}, true, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "3\nout-line\nerr-line" {
		t.Fatalf("output = %q", out)
	}
}

func TestExecDisabledInWebIDE(t *testing.T) {
	_, _, err := runScriptConfig(t, `exec("echo");`, nil, false, nil)
	if err == nil {
		t.Fatal("expected exec to be disabled")
	}
	if !strings.Contains(err.Error(), "disabled in the web IDE") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecCannotFindCommand(t *testing.T) {
	_, _, err := runScriptConfig(t, `exec("veri-command-that-does-not-exist-xyz");`, nil, true, nil)
	if err == nil {
		t.Fatal("expected a missing-command error")
	}
	if !strings.Contains(err.Error(), "cannot run") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecArgumentValidation(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"no args", `exec();`, "takes 1-2 arguments"},
		{"command type", `exec(5);`, "command must be a string"},
		{"args type", `exec("x", "y");`, "must be an array"},
		{"arg element type", `exec("x", [1]);`, "argument 0 must be a string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := runScriptConfig(t, tc.src, nil, true, nil)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestAssertPasses(t *testing.T) {
	_, out, err := runScriptConfig(t, `assert(true); assert(1); print "ok";`, nil, false, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "ok" {
		t.Fatalf("output = %q", out)
	}
}

func TestAssertFailureIsCatchable(t *testing.T) {
	src := `
try {
  assert(false, "boom");
} catch err {
  print err["message"];
}
`
	_, out, err := runScriptConfig(t, src, nil, false, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "assertion failed: boom" {
		t.Fatalf("output = %q", out)
	}
}

func TestReadLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.ToSlash(filepath.Join(dir, "data.txt"))
	if err := os.WriteFile(path, []byte("alpha\nbeta\r\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := fmt.Sprintf(`
let lines = read_lines(%q);
print len(lines);
print lines[0];
print lines[1];
print lines[2] == "";
`, path)
	_, out, err := runScriptConfig(t, src, nil, true, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "3\nalpha\nbeta\ntrue" {
		t.Fatalf("output = %q", out)
	}
}

func TestReadLinesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.ToSlash(filepath.Join(dir, "empty.txt"))
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	src := fmt.Sprintf(`print len(read_lines(%q));`, path)
	_, out, err := runScriptConfig(t, src, nil, true, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "0" {
		t.Fatalf("output = %q, want 0", out)
	}
}

func TestAppendFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.ToSlash(filepath.Join(dir, "log.txt"))
	src := fmt.Sprintf(`
write_file(%q, "a");
append_file(%q, "b");
append_file(%q, "c");
print read_file(%q);
`, path, path, path, path)
	_, out, err := runScriptConfig(t, src, nil, true, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "abc" {
		t.Fatalf("output = %q", out)
	}
}

func TestAppendFileCreates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.ToSlash(filepath.Join(dir, "new.txt"))
	src := fmt.Sprintf(`append_file(%q, "hello"); print read_file(%q);`, path, path)
	_, out, err := runScriptConfig(t, src, nil, true, nil)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out != "hello" {
		t.Fatalf("output = %q", out)
	}
}

func TestFileHelpersDisabledInWebIDE(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"read_lines", `read_lines("x.txt");`},
		{"append_file", `append_file("x.txt", "y");`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := runScriptConfig(t, tc.src, nil, false, nil)
			if err == nil {
				t.Fatal("expected file access to be disabled")
			}
			if !strings.Contains(err.Error(), "disabled in the web IDE") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
