package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func storePath(t *testing.T) string {
	t.Helper()
	return filepath.ToSlash(filepath.Join(t.TempDir(), "app.json"))
}

func TestStoreCRUDAndPersistence(t *testing.T) {
	path := storePath(t)
	src := fmt.Sprintf(`
let db = db_open(%q);
print db_has(db, "users");
db_set(db, "users", [{"name": "Ada"}]);
print db_has(db, "users");
print db_get(db, "users")[0]["name"];
db_set(db, "count", 1);
print db_keys(db);
print db_delete(db, "count");
print db_delete(db, "count");
print db_get(db, "missing");
`, path)
	out, err := runSourceWithIO(t, src, true)
	if err != nil {
		t.Fatal(err)
	}
	want := "false\ntrue\nAda\n[count, users]\ntrue\nfalse\nnull"
	if out != want {
		t.Fatalf("output = %q, want %q", out, want)
	}

	// A second run reopens the same file and sees the persisted value.
	src = fmt.Sprintf(`
let db = db_open(%q);
print db_get(db, "users")[0]["name"];
print len(db_keys(db));
`, path)
	out, err = runSourceWithIO(t, src, true)
	if err != nil {
		t.Fatal(err)
	}
	if out != "Ada\n1" {
		t.Fatalf("reopened store output = %q", out)
	}
}

func TestStoreFileIsJSONAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.ToSlash(filepath.Join(dir, "app.json"))
	src := fmt.Sprintf(`let db = db_open(%q); db_set(db, "answer", 42);`, path)
	if _, err := runSourceWithIO(t, src, true); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"answer": 42`) {
		t.Fatalf("store file = %s", data)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temporary file left behind: %s", entry.Name())
		}
	}
}

func TestStoreValuesAreCopied(t *testing.T) {
	path := storePath(t)
	src := fmt.Sprintf(`
let db = db_open(%q);
let value = [1];
db_set(db, "value", value);
push(value, 2);
print len(db_get(db, "value"));
let fetched = db_get(db, "value");
push(fetched, 3);
print len(db_get(db, "value"));
`, path)
	out, err := runSourceWithIO(t, src, true)
	if err != nil {
		t.Fatal(err)
	}
	if out != "1\n1" {
		t.Fatalf("output = %q, want stored value to be isolated", out)
	}
}

func TestStoreRejectsInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.ToSlash(filepath.Join(dir, "app.json"))
	if err := os.WriteFile(filepath.FromSlash(path), []byte("[1, 2, 3]"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runSourceWithIO(t, fmt.Sprintf(`db_open(%q);`, path), true)
	if err == nil {
		t.Fatal("expected an error for a non-object store")
	}
	if !strings.Contains(err.Error(), "must contain a JSON object") {
		t.Fatalf("error = %v", err)
	}
}

func TestStoreGatedInWebIDE(t *testing.T) {
	path := storePath(t)
	_, err := runSourceWithIO(t, fmt.Sprintf(`db_open(%q);`, path), false)
	if err == nil {
		t.Fatal("expected db_open to be gated")
	}
	if !strings.Contains(err.Error(), "disabled in the web IDE") {
		t.Fatalf("error = %v", err)
	}
}

func TestStoreRejectsUnencodableValues(t *testing.T) {
	path := storePath(t)
	src := fmt.Sprintf(`
let db = db_open(%q);
try {
  db_set(db, "fn", func(x) { return x; });
  print "stored";
} catch e {
  print e["message"];
}
print db_has(db, "fn");
`, path)
	out, err := runSourceWithIO(t, src, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "cannot store this value") {
		t.Fatalf("output = %q", out)
	}
	if !strings.HasSuffix(out, "false") {
		t.Fatalf("failed write should not store a value: %q", out)
	}
}

func TestStoreSharedAcrossTasks(t *testing.T) {
	path := storePath(t)
	src := fmt.Sprintf(`
let db = db_open(%q);

func writerA() {
  loop i in range(1, 15) {
    db_set(db, "a" + str(i), i);
  }
}

func writerB() {
  loop i in range(1, 15) {
    db_set(db, "b" + str(i), i);
  }
}

let t1 = spawn writerA();
let t2 = spawn writerB();
await t1;
await t2;
print len(db_keys(db));
print db_get(db, "a10");
print db_get(db, "b14");
`, path)
	out, err := runSourceWithIO(t, src, true)
	if err != nil {
		t.Fatal(err)
	}
	if out != "28\n10\n14" {
		t.Fatalf("output = %q", out)
	}
}
