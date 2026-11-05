# VeriCode

**0.7 alpha** — a small, evolving programming language written in Go.

VeriCode starts as a clear, approachable language for learning and scripting. The long-term aim is to make **concurrency, parallelism, and data-oriented work** first-class and practical — so programs can grow from simple scripts into concurrent and data-heavy workloads without drowning in boilerplate.

**Pipeline today:** Lexer → Parser → Compiler → Bytecode VM

**Direction:** sequential core → concurrent runtime → parallel data transforms → scalable data tools

---

## Vision

Many languages support concurrency and large-scale data processing, but they often demand a steep jump in complexity. VeriCode's goal is different in *emphasis*, not in denying what others already do well:

* **Concurrency that stays readable** — tasks and communication as a natural part of the language
* **Parallelism for data work** — parallel maps, pipelines, and batch transforms over collections
* **A path from small to serious** — same language from `print "hello"` to concurrent and data-oriented programs
* **Safe defaults** — design toward avoiding data races and surprising shared-state bugs

**0.1** was the foundation: a solid sequential language and tooling.

**0.2** added the first concurrent layer: lightweight tasks, channels, and a clear spawn/await model — plus source-located error messages with call stacks.

**0.3** added the parallel layer: `pmap`, `pfilter`, and `pfor` run callbacks on a pool of isolated worker VMs, with order-preserving results.

**0.4** added the data layer: pipe-friendly batch transforms, table-like structures, CSV/JSON, and file I/O for the CLI.

**0.5** adds the scripting layer: `try` / `catch` / `throw` with structured error values, script plumbing (`args()`, `env()`, `now()`, `random()`), and modules with `import name from "path.veri"`.

**0.6** adds the application layer: `serve(port, routes)` for small HTTP APIs and `db_open` for file-backed JSON stores that survive restarts.

**0.7** adds the tooling layer: a check / eval / REPL toolchain, `assert`, a gated native command hook (`exec`), line-oriented file helpers, and table joins (`tjoin`).

---

## Current status (0.7 alpha)

Usable for demos, learning, sequential scripts, multi-file command-line tools, cooperative concurrency, parallel collection work, small data pipelines over CSV/JSON, programs that recover from failures, interactive exploration through the REPL, and small HTTP APIs backed by a persistent store. **Not** production-hardened, and not a big-data engine: everything fits in memory.

### What works today

|Area|Support|
|-|-|
|**Types**|integers, floats (including scientific `1e3`), booleans, strings, chars, arrays, hashes, tables, `null`, functions|
|**Control**|`if` / `else`, `loop while`, `loop n times`, `loop x in arr`, `break` / `continue`|
|**Errors**|`try` / `catch` / `throw`, structured error values (`message`, `type`, `file`, `line`, `col`, `trace`)|
|**Functions**|first-class, closures, recursion, named top-level `func name(...) { }`|
|**Operators**|arithmetic, comparison, logical, compound assign (`+=` and friends), pipe operator, index and slice|
|**Strings**|escapes, `${expr}` interpolation, `format("{}", ...)`|
|**Collections**|arrays and hashes, index read/write, slicing (`arr[1:3]`, `str[:2]`), batch transforms|
|**Stdlib**|`len`, `push` / `pop`, `map` / `filter` / `reduce`, `range`, math (`abs`, `min`, `max`, `sqrt`, `pow`), string (`split`, `join`, `trim`, `contains`, `replace`, `format`), `assert`|
|**Concurrency**|`spawn` tasks, `await` join, channels (`chan` / `send` / `recv`), `sleep`, snapshot-isolated task state, deadlock detection|
|**Parallelism**|`pmap` / `pfilter` / `pfor` on a worker pool (size via optional arg or `workers()`), order-preserving results, callback errors with full source context|
|**Data**|pipelines with `|>`, `take` / `drop` / `chunk` / `zip` / `uniq` / `group_by` / `sort_by` and friends, tables (`table`, `tselect`, `twhere`, `tsort`, `tgroup`, `tjoin`)|
|**I/O**|`csv_parse` / `csv_table` / `csv_write`, `json_parse` / `json_write`, `read_file` / `write_file` / `read_lines` / `append_file`, and `exec` in CLI runs only|
|**Scripting**|`args()` and `env(name)` in CLI runs only, Unix-millisecond `now()`, `random()` / `random(n)` / `random(a, b)`, line input with `ask`|
|**Modules**|`import name from "path.veri"`, namespaced exports, paths relative to the importing file, one run per module, circular-import detection (CLI only)|
|**HTTP**|`serve(port, routes)` with `"METHOD /path"` routes, `:params`, request/response hashes, status/headers/JSON bodies, serialized handler execution (CLI only)|
|**Persistence**|`db_open` / `db_get` / `db_set` / `db_delete` / `db_has` / `db_keys` over an atomically written JSON file, shared across tasks (CLI only)|
|**Tooling**|`veri -run`, `veri -check`, `veri -e`, persistent `veri -repl`, `veri -version`, web IDE (Monaco), source-aware error messages|

---

## Roadmap

|Phase|Theme|Direction|
|-|-|-|
|**0.1**|Core language|Types, functions, slices, stdlib, CLI and web IDE|
|**0.2**|Concurrency|Lightweight tasks, message passing / channels, clear spawn and join model|
|**0.3**|Parallelism|Parallel collection ops, worker pools, CPU-bound parallel loops|
|**0.4**|Data-oriented|Pipelines, batch transforms, practical I/O (CSV/JSON), table-like structures|
|**0.5**|Scripting|`try` / `catch` / `throw`; CLI arguments, environment, clock, randomness; modules and imports|
|**0.6**|Applications|Small HTTP APIs with `serve`, file-backed key-value persistence with `db_open`|
|**0.7** (now)|Tooling|Check / eval / REPL toolchain, assertions, gated native command hook, line-oriented file helpers, table joins|

This roadmap is intentional: **concurrency before big data**. A trustworthy concurrent runtime is the base for parallel and data features, and the scripting and application layers build on the same runtime.

All roadmap phases are implemented. Future work is maintenance, documentation, and refinement of the 0.7 behavior rather than a new phase.

---

## Requirements

* [Go 1.22+](https://go.dev/dl/)

---

## Build

```bash
git clone https://github.com/Rohtihjain/Vericode.git
cd Vericode
go build -o veri .
```

Windows (PowerShell):

```powershell
go build -o veri.exe .
```

---

## Run

```bash
# Run a source file (file I/O enabled)
./veri -run examples/hello.veri

# Parse and compile only — diagnostics, no execution
./veri -check examples/hello.veri

# Run an inline snippet
./veri -e 'print 2 + 3;'

# Interactive session with persistent variables (:help, :reset, :quit)
./veri -repl

# Web IDE at http://localhost:8080 (file I/O disabled)
./veri -web

# Print the version
./veri -version

# Show usage
./veri
```

On Windows use `.\veri.exe` instead of `./veri`.

Source files conventionally use the `.veri` extension.

`-run` and `-e` print each line as the program produces it, so servers and long loops show progress instead of buffering everything until exit.

---

## Quick examples (0.1)

### Hello and interpolation

```veri
let name = "Ada";
print "Hello, ${name}!";
print format("score = {}", 95);
```

### Named function and recursion

```veri
func fact(n) {
  if n <= 1 { return 1; }
  return n * fact(n - 1);
}
print fact(5);
```

### Slices and functional tools

```veri
let nums = range(0, 10);
print nums[2:5];
print "hello"[1:4];

print map(nums, func(n) { return n * n; });
print 5 |> func(x) { return x * x; };
```

### Floats and math

```veri
print 1e3;
print 2.5e-1;
print sqrt(16.0);
print pow(2, 10);
```

### Tasks and channels

```veri
let ch = chan(3);            // buffered channel (cap 3); chan() = unbuffered

let worker = func(name, n) {
  loop i in range(0, n) {
    send(ch, name + str(i)); // blocks when the buffer is full
  }
};

let t1 = spawn worker("a", 3); // runs on its own VM, in its own goroutine
let t2 = spawn worker("b", 3);

let results = [];
loop 6 times {
  results = push(results, recv(ch)); // receive BEFORE awaiting producers
}
await t1; // join: waits for the task, returns its result, re-raises its error
await t2;

print len(results);
```

---

## Concurrency (0.2)

VeriCode's concurrency is **share-nothing with explicit sharing**:

* **`spawn f(args)`** starts a task running `f` on its own VM in its own goroutine, and returns a task handle immediately.
* **Snapshot isolation** — the task receives a *copy* of the parent's variables: arrays, hashes, tables, and closures are deep-copied at spawn. A task cannot mutate the parent's state, so **there are no data races by construction**.
* **Channels are the one shared thing.** `chan(n)` creates a buffered channel, `chan()` an unbuffered (rendezvous) one. `send(ch, v)` / `recv(ch)` move values between tasks; values pass through the channel itself.
* **`await t`** joins a task: it waits for completion, returns the task's result, and re-raises the task's error at the `await` site.
* **Errors surface even if you forget to join** — an unawaited task's failure ends the run with that error. An error re-raised at an `await` site counts as handled once `try` / `catch` catches it.
* **Cancellation is tree-shaped.** If the main program fails, it cancels every task it spawned (and they cancel theirs). Blocked `send` / `recv` / `await` / `sleep` wake immediately with a cancellation error instead of leaking goroutines.
* **`sleep(ms)`** pauses only the current task.
* **Deadlocks are detected.** When every task is blocked in an untimed wait — `send`, `recv`, `await`, or a pool join — and no `sleep` timer is pending, the run stops with a source-located `deadlock` error that lists who is waiting on what. No more Go `fatal error` in the CLI or a hung request in the web IDE.
* Print output from all tasks is collected on one shared, mutex-guarded stream — line order is scheduling order and intentionally unspecified.

### Known limitations

* **Detection covers VeriCode-level waits only.** A task blocked in external I/O (stdin via `ask`, file reads) is not considered deadlocked, because that input may still arrive.
* Tasks are **not** killed mid-computation — cancellation is checked only at `send` / `recv` / `await` / `sleep`.
* Concurrency is designed to be data-race free (snapshot isolation + shared-nothing VMs), but run `go test -race ./...` on a machine with a C compiler to verify on other platforms.

---

## Parallelism (0.3)

Parallel collection operations run your callback on a **pool of worker VMs** — the same snapshot-isolated, share-nothing design as `spawn`, but structured: the pool waits for all workers before returning, so results can never outlive the call.

* **`pmap(arr, fn, [workers])`** maps in parallel; **`pfilter(arr, fn, [workers])`** filters in parallel. Results always keep the input order, whatever the scheduling.
* **`pfor(start, end, fn, [workers])`** runs `fn(i)` for every `i` in `start..end-1` in parallel and returns the results in index order — for CPU-bound loops it skips building the input array that `pmap(range(...), fn)` would.
* **Worker pool** — workers pull indices from a shared counter, so slow elements don't strand a worker. The pool size defaults to `workers()` (one per CPU) and can be set per call (capped at 64).
* **Isolation** — each worker runs on its own snapshot of the variables, like a spawned task. Callbacks can `print` (shared console) and use channels, but cannot mutate the caller's state. There are no data races by construction.
* **Errors carry the full causal chain** — a failing callback aborts the pool and reports the call site, the worker's own source location, and its call stack. A blocked callback wakes immediately on cancellation, including when an ancestor task fails.
* Nested parallelism works: a `pmap` callback may itself run `pmap` / `pfilter` / `pfor`.

```veri
func fib(n) {
  if n < 2 { return n; }
  return fib(n - 1) + fib(n - 2);
}

// CPU-bound loop spread across all cores, results in index order
let fibs = pfor(0, 26, func(i) { return fib(i); });
print fibs;

// Two-worker pool, order preserved
print pmap([1, 2, 3], func(x) { return x * 10; }, 2);
```

---

## Data (0.4)

### Pipelines

`|>` feeds the value on its left into the **first argument** of the call on its right:

```veri
nums |> take(3)   // take(nums, 3)
xs |> sum         // sum(xs)
```

That makes every collection builtin below pipe-friendly, since they all take the collection first. Chains read top to bottom:

```veri
let total = range(1, 21)
  |> filter(func(x) { return x % 2 == 0; })
  |> map(func(x) { return x * x; })
  |> take(4)
  |> sum;
print total; // 4 + 16 + 36 + 64 = 120
```

### Batch transforms

|Builtin|Result|
|-|-|
|`take(arr, n)` / `drop(arr, n)`|first `n` / all but the first `n`|
|`chunk(arr, n)`|array of arrays of size `n` (last chunk may be shorter)|
|`flatten(arr)`|one level deep: array elements are spliced in, other values kept|
|`zip(a, b)`|pairs `[a[i], b[i]]` up to the shorter length|
|`enumerate(arr)`|pairs `[index, value]`|
|`uniq(arr)`|first occurrences, comparing arrays/hashes structurally|
|`sum(arr)` / `avg(arr)`|numeric total / mean (`sum` keeps INTEGER when all values are integers)|
|`min(arr)` / `max(arr)`|also accept a single array; variadic form still works|
|`sort(arr)` / `sort_by(arr, fn)`|stable; numbers, strings, booleans; `null` sorts last|
|`reverse(arr)`|reversed copy|
|`any(arr, [fn])` / `all(arr, [fn])`|truthiness, or the callback's result; short-circuits|
|`group_by(arr, fn)`|hash from key to array of elements|
|`pluck(arr, column)`|values of a column from an array of hashes (or a table)|
|`contains(arr, value)`|structural membership (strings still do substring search)|

Transform callbacks run sequentially in the calling VM; use `pmap` / `pfilter` when the work should use multiple cores.

Builtin names are ordinary identifiers: `let sum = 0;` shadows the `sum` builtin in that scope.

### Tables

A table is rows of hashes plus an ordered header list. Rows are real hashes, so cells can be updated in place.

```veri
let people = table([
  {"name": "Ada", "age": 36, "city": "London"},
  {"name": "Bob", "age": 5,  "city": "Paris"},
  {"name": "Cy",  "age": 41, "city": "London"}
], ["name", "age", "city"]);

print people;              // aligned columns
print len(people);         // 2 rows
print columns(people);     // [name, age, city]
print people[0]["name"];   // first row, one cell
print people["city"];      // whole column
people[0]["age"] = 37;     // rows are mutable hashes
```

* **`table(rows, [headers])`** builds from an array of hashes. Without explicit headers the union of keys is used in **sorted order** (deterministic). With explicit headers, the listed order wins, missing cells become `null`, and row keys outside the header list are an error.
* **`tselect(t, cols...)`** projects columns, **`twhere(t, fn)`** keeps rows where `fn(row)` is truthy, **`tsort(t, col, [asc|desc])`** sorts by a column (stable, `null` last), **`tgroup(t, col)`** returns a hash from column value to a sub-table.
* **`tjoin(left, right, key, [mode])`** joins two tables on a shared column. The default is an inner join; pass `"left"` to keep unmatched left rows with null right-side cells. Right-side columns that collide with left-side names get a `_right` suffix. Matching uses the same equality as hash keys, so integers and floats are distinct.
* **`rows(t)`** exposes the row hashes; **`pluck(t, col)`** extracts a column. `has(t, "col")` reports whether a column exists.
* Tables are snapshot-isolated across `spawn` and parallel workers, exactly like arrays and hashes.

### CSV

```veri
let text = "name,score\nAda,95\nBob,88\n";

let rows = csv_parse(text);     // array of arrays of strings
let t = csv_table(text);        // first row becomes headers

print t;                        // aligned table
print csv_write(t);             // headers + rows back to CSV
print csv_write([[1, "a,b"]]);  // arrays also work, with quoting
```

CSV has no types: every cell is a string (use `int()`, `sum`, etc.). Short rows are padded with empty strings; rows with more fields than the header are an error. Quoting, embedded commas, and embedded newlines are handled by the standard CSV rules.

### JSON

```veri
let cfg = json_parse("{\"debug\": true, \"retries\": 3, \"tags\": [\"x\"]}");
print cfg["retries"] + 1;       // numbers arrive as INTEGER / FLOAT
print json_write(cfg);          // compact
print json_write(cfg, 2);       // two-space indent
```

* `json_parse` maps objects to hashes, arrays to arrays, integral numbers to INTEGER, decimal/exponent numbers to FLOAT, and `true` / `false` / `null` to the matching values.
* `json_write` handles integers, floats, booleans, strings, `null`, arrays, hashes (string keys), and tables (encoded as an array of row objects).
* Functions, channels, and tasks cannot be encoded; cyclic values, non-finite floats, and non-string hash keys are reported as errors instead of crashing.

### Files (CLI only)

```veri
write_file("out.json", json_write(cfg, 2));
print read_file("out.json");

let scores = csv_table(read_file("scores.csv"));
write_file("totals.csv", csv_write(scores));
```

`read_file(path)` returns the whole file as a string; `write_file(path, text)` truncates or creates it. **`read_lines(path)`** returns an array of lines without terminators (CRLF is normalized) and **`append_file(path, text)`** creates or appends in place. Paths are relative to the current working directory.

**File access is disabled in the web IDE**: programs posted to `/api/run` get a clear `read_file() is disabled in the web IDE` error, while `veri -run` enables it. The flag is inherited by spawned tasks and parallel workers.

### Script plumbing (0.5)

```veri
// veri -run report.veri sales.csv 2026
let cli = args();
print len(cli);              // 2
print cli[0];                // sales.csv

let who = env("USER");       // string, or null when unset
print who == null;

print now();                 // Unix milliseconds
print random();              // float in [0, 1)
print random(6);             // integer in [0, 6)
print random(1, 7);          // integer in [1, 7)

let name = ask "Your name: ";
print "Hi, " + name;
```

* `args()` returns everything after the script path as an array of strings. It is empty in the web IDE. Pass dash-prefixed arguments after `--`.
* `env(name)` returns the variable as a string, or `null` when it is unset. **Like file I/O, it is CLI-only**: the web IDE cannot read the host environment.
* `now()` returns Unix time in milliseconds as an integer; `random()` gives a float in `[0, 1)`, `random(n)` an integer in `[0, n)`, and `random(a, b)` an integer in `[a, b)`.
* `ask` reads one line from standard input per call; successive calls consume successive lines, and concurrent tasks share the input safely.

### Limitations

* Everything is in memory — no streaming or huge-file processing yet.
* CSV loses types; JSON object keys are strings and written keys are sorted.
* Tables are a view over rows of hashes: `tjoin` covers inner and left joins, but there are no indexes. Use `rows(t) |> map(...) |> group_by(...)` for custom aggregation, and `db_set` to persist a table across runs.

---

## Error handling (0.5)

```veri
try {
  let parsed = json_parse(input);
  print parsed["name"];
} catch err {
  print "failed: " + err["message"];
  print "at " + err["file"] + ":" + str(err["line"]);
}
```

* **`try { ... } catch name { ... }`** runs the body; if a statement raises, the rest of the body is skipped and execution continues in the handler. With no error, the handler never runs.
* **`throw value;`** raises an error deliberately. A string becomes the message; any other value is rendered with its printed form.
* **The catch variable is a hash** describing the failure:

  |Key|Meaning|
  |-|-|
  |`message`|the error message, such as `"division by zero"`|
  |`type`|the error kind (`"RuntimeError"`)|
  |`file`, `line`, `col`|where the error was raised|
  |`trace`|call stack text; empty when raised in the main program|

* **Anything can fail** — arithmetic (`1 / 0`), index and slice misuse, builtin argument errors, callback failures from `pmap` / `pfilter` / `pfor`, and errors re-raised by `await` all land in the nearest active handler.
* **Handlers unwind across calls.** A `try` in the caller catches an error raised several frames deep: call frames and stack temporaries are discarded, and execution resumes at the handler.
* **`return`, `break`, and `continue` leave no stale handlers behind**, whether they exit a try directly or from inside a nested loop.
* **An error raised inside a handler propagates to the enclosing handler**, so nested tries can add context and rethrow.
* **Errors are ordinary hash values**, so they can be printed, indexed, encoded with `json_write`, or passed to functions.
* **Some failures are not catchable.** Cancellation and deadlock detection are fatal by design: catching a cancellation would let cancelled work keep running, and catching a deadlock would leave the runtime stuck. They always end the run.
* **A caught `await` failure counts as handled** — once `await` re-raises a failed task's error, the runtime no longer reports that task a second time as an unawaited failure.

Uncaught errors keep the existing diagnostics: the source line with a caret, the error type, and the call stack.

---

## Modules (0.5)

```veri
// lib/math.veri
let PI = 3.14159;
func add(a, b) { return a + b; }

// main.veri
import math from "lib/math.veri";
print math["PI"];            // 3.14159
print math["add"](2, 3);     // 5
```

* **`import name from "path.veri";`** runs the module once and binds `name` to a hash of its top-level names. Access exports with `name["member"]`; exported functions can be called directly: `math["add"](2, 3)`.
* **Every top-level binding is exported** — `let`, `func`, and loop variables alike. Private helpers are those you keep local to a function.
* **Paths are relative to the importing file**, not the working directory. Modules can import other modules the same way, resolved from their own directory.
* **A module runs once per run** even if several files import it; its `print` output lands in the main output, and side effects happen at first import.
* **Imports are CLI-only**, gated like file I/O: the web IDE has no filesystem, so it reports `import is disabled in the web IDE`.
* **Failures are imported with context** — parse, compile, and runtime errors inside a module carry the module's own file, line, and source snippet. The import statement reports the cause, and `try` / `catch` can handle import failures.
* **Circular imports are rejected** with a clear `circular import` error rather than recursing forever.
* **Exported functions work everywhere** — inside spawned tasks and parallel callbacks, with their module globals intact.

---

## Applications (0.6)

### HTTP APIs

```veri
func hello(req) {
  return {
    "status": 200,
    "headers": {"X-Powered-By": "VeriCode"},
    "body": "hello " + req["params"]["name"],
  };
}

func createNote(req) {
  let notes = [];
  if db_has(db, "notes") {
    notes = db_get(db, "notes");
  }
  notes = push(notes, {"id": len(notes) + 1, "text": req["body"]});
  db_set(db, "notes", notes);
  return {"status": 201, "body": notes[len(notes) - 1]};
}

let routes = {
  "GET /hello/:name": hello,
  "POST /notes": createNote,
};

serve(8080, routes);
```

`serve(port, routes)` binds an HTTP server and blocks until the process stops. Each route key is `"METHOD /path"`; method names are case-insensitive, and a `:name` segment captures one path value. Trailing slashes are ignored, and routes with more literal segments win over patterns such as `/:id`.

* **The request hash** has `method`, `path`, `params`, `query`, `headers`, and `body`. `params` holds `:name` captures, while `query` and `headers` hold the first value of each name.
* **The response is the handler's return value.** A string is sent as `text/plain`; arrays, tables, numbers, booleans, and hashes are JSON-encoded with `application/json`. Return `null` for an empty body.
* **Return an envelope to control the response**: a hash with `status`, `headers`, and/or `body` sets those parts. Headers must be a hash of strings; `status` must be an integer.
* **Failures are HTTP 500s.** An uncaught error (including `throw`) returns `500` with the diagnostic, so one bad request does not stop the server. Unknown paths return `404`; a known path with the wrong method returns `405` and an `Allow` header.
* **Handlers run one at a time** on the program's VM, one request after another. That keeps ordinary globals safe between requests; use `spawn` inside a handler only for work you want to continue after the response.
* **`print` inside a handler** writes to the normal program output, which streams to the CLI while the server runs.
* **The server is deliberately small**: no TLS, no middleware, no graceful shutdown (Ctrl+C stops it), and no request-size limit beyond 8 MiB.

Like file I/O, `serve` is **CLI-only**: the web IDE reports `serve() is disabled in the web IDE`.

### Persistence

```veri
let db = db_open("app-store.json");

db_set(db, "visits", 1);
print db_get(db, "visits");   // 1
print db_has(db, "visits");   // true
print db_keys(db);            // [visits]
print db_delete(db, "visits");// true
print db_get(db, "visits");   // null
```

`db_open(path)` opens a JSON object on disk as a small key-value store. A missing file is created; an existing file must contain a JSON object.

|Function|Result|
|-|-|
|`db_get(db, key)`|the stored value, or `null` when the key is absent|
|`db_set(db, key, value)`|stores a JSON-compatible value and saves the file|
|`db_delete(db, key)`|`true` if the key existed, `false` otherwise|
|`db_has(db, key)`|`true` when the key exists|
|`db_keys(db)`|a sorted array of keys|

* **Values are JSON-compatible**: integers, floats, booleans, strings, `null`, arrays, hashes, and tables (as arrays of row objects). Functions, channels, tasks, and stores are rejected with a catchable error.
* **Every change is written atomically** (a temporary file is renamed over the store), so an interrupted run never leaves half a file. The whole store stays in memory, so reads are immediate.
* **Values are copied in and out.** Mutating an array returned by `db_get` does not change the store until you `db_set` it again.
* **One store is shared, not snapshotted.** Spawned tasks and server handlers all see the same data, and the store's own lock serializes concurrent writes. This is the intended way to share mutable state between tasks.
* **`db_open` is CLI-only**, gated like the other filesystem functions. Relative paths follow the working directory.

---

## Tooling (0.7)

### CLI

```bash
veri -check app.veri        # parse + compile, print diagnostics, exit 1 on failure
veri -e 'print 42;'         # run inline source
veri -repl                  # persistent interactive session
veri -version
```

`-check` is useful in editors and CI: it reports parse and compile problems with the usual file, line, and caret without running the program.

The REPL compiles every line into one persistent global table, so definitions survive between inputs:

```
veri> let x = 2;
veri> x + 3
5
veri> func double(n) {
...    return n * 2;
... }
veri> double(21)
42
```

* Bare expressions echo their value; statements that produce `null` stay quiet.
* Multi-line blocks continue while braces are open.
* An error prints its diagnostic and leaves the session alive.
* `:help`, `:reset` (forget all names), and `:quit` manage the session.
* Imports are not available in the REPL because runtime module globals would collide with later input lines; run a file with `veri -run`.
* Spawned tasks are joined before the next prompt.

### Assertions

```veri
func average(xs) {
  assert(len(xs) > 0, "average() needs at least one value");
  return sum(xs) / len(xs);
}
```

`assert(condition, [message])` does nothing when the condition is truthy and raises a catchable runtime error otherwise. The message is optional; non-string values are rendered with their printed form.

### Native commands (CLI only)

```veri
let r = exec("git", ["rev-parse", "--short", "HEAD"]);
print r["code"];          // 0
print trim(r["stdout"]);  // commit hash
print r["stderr"];        // captured separately
```

`exec(command, [args])` runs a program directly — **no shell**, so there is no shell-injection surface. It returns a hash with `code`, `stdout`, and `stderr`. A non-zero exit is a normal result, not an error; a command that cannot start raises instead. Like file I/O and imports, `exec` is **disabled in the web IDE**, where it reports `exec() is disabled in the web IDE`.

### File helpers (CLI only)

`read_lines(path)` reads a text file as an array of lines without terminators, and `append_file(path, text)` creates or appends without rewriting the whole file:

```veri
append_file("run.log", "started\n");
loop line in read_lines("scores.csv") |> drop(1) {
  print line;
}
```

Both use the same CLI-only gate as `read_file` / `write_file`.

---

## Project layout

```
.
├── ast.go           # Abstract syntax tree
├── builtins.go      # Native functions and registration
├── code.go          # Opcodes
├── codec.go         # CSV, JSON, gated file I/O and line/append helpers
├── compiler.go      # AST to bytecode (with source maps)
├── concurrency.go   # Task spawning, snapshot isolation, shared output
├── data.go          # Pipelines and batch transforms
├── deadlock.go      # Deadlock detection for blocked tasks
├── diagnostic.go    # Error formatting
├── lexer.go         # Tokenizer
├── main.go          # CLI and embedded web server
├── module.go        # Module loading, compilation, caching, exports
├── native.go        # assert and the gated native command hook
├── object.go        # Runtime values
├── parallel.go      # Worker pool, pmap / pfilter / pfor
├── parser.go        # Pratt parser
├── persistence.go   # File-backed JSON key-value store
├── repl.go          # Persistent interactive session
├── script.go        # CLI args, environment, clock, randomness
├── serve.go         # HTTP server, route matching, request/response mapping
├── table.go         # Table type and table builtins
├── token.go         # Token types and keywords
├── vm.go            # Bytecode interpreter
├── vm_test.go       # Core language and error-reporting tests
├── try_test.go      # try / catch / throw tests
├── script_test.go   # args / env / now / random and ask tests
├── module_test.go   # Import loading, caching, cycles, and gating tests
├── native_test.go   # assert / exec / read_lines / append_file tests
├── persistence_test.go # Store CRUD, copying, sharing, and gating tests
├── serve_test.go    # Routing, responses, errors, and gating tests
├── tool_test.go     # Check, eval, and REPL session tests
├── parallel_test.go # Parallel-op tests
├── data_test.go     # Pipeline and transform tests
├── deadlock_test.go # Deadlock detection tests
├── table_test.go    # Table tests
├── codec_test.go    # CSV, JSON, and file I/O tests
├── examples/        # Sample .veri programs
└── web/
    └── index.html   # VeriCode Studio (Monaco)
```

---

## Tests

```bash
go test ./...

# Concurrency is covered by the race detector on platforms with a C compiler:
go test -race ./...
```

---

## Web IDE

```bash
./veri -web
```

Open [http://localhost:8080](http://localhost:8080).

* **Examples** dropdown — curated samples, including **Scripting**, **Data**, and **Joins**; `examples/app.veri` (HTTP + persistence) runs from the CLI
* **Click the filename** to rename before save
* **Ctrl+Enter** — run
* **Ctrl+S** — download `.veri` file
* **Open** — load a file from disk

Programs run on the server via `POST /api/run`. File access, environment variables, imports, `serve`, persistence, and native commands are disabled there.

---

## Language sketch (0.7)

```veri
let x = 10;
x += 1;

if x > 5 {
  print "big";
} else {
  print "small";
}

loop while x > 0 {
  x -= 1;
}

loop item in [1, 2, 3] {
  print item;
}

func greet(name) {
  print "Hi, ${name}";
}
greet("World");

let h = {"a": 1};
h["b"] = 2;
print keys(h);

// concurrency
let ch = chan();
let t = spawn func() {
  send(ch, 42);
  return "done";
}();
print recv(ch);
print await t;

// parallelism — order-preserving results on a worker pool
let squares = pmap(range(1, 6), func(n) { return n * n; });
print squares;              // [1, 4, 9, 16, 25]

let fibs = pfor(0, 10, func(i) { return i * 2; }, 4);
print fibs;                 // [0, 2, 4, 6, 8, 10, 12, 14, 16, 18]

// data — pipelines, tables, CSV, JSON
let evens = range(1, 11)
  |> filter(func(n) { return n % 2 == 0; })
  |> map(func(n) { return n * n; });
print evens;                // [4, 16, 36, 64, 100]

let people = table([
  {"name": "Ada", "age": 36},
  {"name": "Bob", "age": 5}
], ["name", "age"]);
print people |> tsort("age", "desc") |> tselect("name");

let csv = csv_write(people);
print json_write(people);

// errors — recover from anything the runtime raises
try {
  print 10 / 0;
} catch err {
  print "caught: " + err["message"];
  print "line " + str(err["line"]);
}

// scripting — CLI arguments, environment, clock, randomness
print args();                 // everything after the script path
print env("USER");            // string or null (CLI only)
let started = now();          // Unix milliseconds
print random();               // [0, 1)
print random(1, 7);           // [1, 7)

// modules — top-level names become a namespace hash (CLI only)
import math from "lib/math.veri";
print math["PI"];
print math["add"](2, 3);

// scale and polish — joins, assertions, native commands
let scores = table([
  {"name": "Ada", "score": 95},
  {"name": "Bob", "score": 88}
], ["name", "score"]);
print tjoin(people, scores, "name", "left");
assert(len(scores) == 2, "expected two scores");

// exec() is CLI-only: run a command and read its result
let git = exec("git", ["--version"]);
print trim(git["stdout"]);
```

---

## Contributing

Ideas, issues, and pull requests are welcome — especially bugs, tests, examples, and documentation. The 0.1–0.7 roadmap is complete, so larger changes should focus on keeping the documented behavior stable rather than adding new phases.

Before proposing large features, open an issue so the design can stay consistent with the roadmap.

---

## License

MIT. See the `LICENSE` file at the repository root.

---

## Credits

Implemented in Go as a classic bytecode language: lexer, Pratt parser, compiler, and stack VM.
