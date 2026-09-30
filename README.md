# VeriCode

**0.2 alpha** — a small, evolving programming language written in Go.

VeriCode starts as a clear, approachable language for learning and scripting. The long-term aim is to make **concurrency, parallelism, and data-oriented work** first-class and practical — so programs can grow from simple scripts into concurrent and data-heavy workloads without drowning in boilerplate.

**Pipeline today:** Lexer → Parser → Compiler → Bytecode VM

**Direction:** sequential core → concurrent runtime → parallel data transforms → scalable data tools

\---

## Vision

Many languages support concurrency and large-scale data processing, but they often demand a steep jump in complexity. VeriCode's goal is different in *emphasis*, not in denying what others already do well:

* **Concurrency that stays readable** — tasks and communication as a natural part of the language
* **Parallelism for data work** — parallel maps, pipelines, and batch transforms over collections
* **A path from small to serious** — same language from `print "hello"` to concurrent and data-oriented programs
* **Safe defaults** — design toward avoiding data races and surprising shared-state bugs

**0.1** was the foundation: a solid sequential language and tooling.

**0.2** adds the first concurrent layer: lightweight tasks, channels, and a clear spawn/await model.

**Later phases** add the parallel and data layers on top of that foundation.

\---

## Current status (0.2 alpha)

Usable for demos, learning, sequential scripts, and cooperative concurrency. **Not** production-hardened. Parallelism and big-data features are **planned**, not shipped yet.

### What works today

|Area|Support|
|-|-|
|**Types**|integers, floats (including scientific `1e3`), booleans, strings, chars, arrays, hashes, `null`, functions|
|**Control**|`if` / `else`, `loop while`, `loop n times`, `loop x in arr`, `break` / `continue`|
|**Functions**|first-class, closures, recursion, named top-level `func name(...) { }`|
|**Operators**|arithmetic, comparison, logical, compound assign (`+=` and friends), pipe operator, index and slice|
|**Strings**|escapes, `${expr}` interpolation, `format("{}", ...)`|
|**Collections**|arrays and hashes, index read/write, slicing (`arr\\\[1:3]`, `str\\\[:2]`)|
|**Stdlib**|`len`, `push` / `pop`, `map` / `filter` / `reduce`, `range`, math (`abs`, `min`, `max`, `sqrt`, `pow`), string (`split`, `join`, `trim`, `contains`, `replace`, `format`)|
|**Concurrency**|`spawn` tasks, `await` join, channels (`chan` / `send` / `recv`), `sleep`, snapshot-isolated task state|
|**Tooling**|CLI runner, web IDE (Monaco), source-aware error messages|

\---

## Roadmap

|Phase|Theme|Direction|
|-|-|-|
|**0.1**|Core language|Types, functions, slices, stdlib, CLI and web IDE|
|**0.2** (now)|Concurrency|Lightweight tasks, message passing / channels, clear spawn and join model|
|**0.3**|Parallelism|Parallel collection ops, worker pools, CPU-bound parallel loops|
|**0.4**|Data-oriented|Pipelines, batch transforms, practical I/O (CSV/JSON), table-like structures|
|**Later**|Scale and polish|Richer runtime, optional native hooks, tooling — only as the core stays solid|

This roadmap is intentional: **concurrency before big data**. A trustworthy concurrent runtime is the base for parallel and data features.

\---

## Requirements

* [Go 1.22+](https://go.dev/dl/)

\---

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

\---

## Run

```bash
# Run a source file
./veri -run examples/hello.veri

# Web IDE at http://localhost:8080
./veri -web

# Show usage
./veri
```

On Windows use `.\\\\veri.exe` instead of `./veri`.

Source files conventionally use the `.veri` extension.

\---

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
  return n \\\* fact(n - 1);
}
print fact(5);
```

### Slices and functional tools

```veri
let nums = range(0, 10);
print nums\\\[2:5];
print "hello"\\\[1:4];

print map(nums, func(n) { return n \\\* n; });
print 5 |> func(x) { return x \\\* x; };
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

\---

## Concurrency (0.2)

VeriCode's concurrency is **share-nothing with explicit sharing**:

* **`spawn f(args)`** starts a task running `f` on its own VM in its own goroutine, and returns a task handle immediately.
* **Snapshot isolation** — the task receives a *copy* of the parent's variables: arrays, hashes, and closures are deep-copied at spawn. A task cannot mutate the parent's state, so **there are no data races by construction**.
* **Channels are the one shared thing.** `chan(n)` creates a buffered channel, `chan()` an unbuffered (rendezvous) one. `send(ch, v)` / `recv(ch)` move values between tasks; values pass through the channel itself.
* **`await t`** joins a task: it waits for completion, returns the task's result, and re-raises the task's error at the `await` site.
* **Errors surface even if you forget to join** — an unawaited task's failure ends the run with that error.
* **Cancellation is tree-shaped.** If the main program fails, it cancels every task it spawned (and they cancel theirs). Blocked `send` / `recv` / `await` / `sleep` wake immediately with a cancellation error instead of leaking goroutines.
* **`sleep(ms)`** pauses only the current task.
* Print output from all tasks is collected on one shared, mutex-guarded stream — line order is scheduling order and intentionally unspecified.

### Known limitations

* **True deadlocks are not detected.** If every task blocks forever (e.g. awaiting producers while their sends fill a buffer nobody drains), the CLI aborts with Go's runtime `fatal error: all goroutines are asleep - deadlock!`, and in the web IDE the run request hangs until the browser gives up. Design programs so some task is always receiving.
* Tasks are **not** killed mid-computation — cancellation is checked only at `send` / `recv` / `await` / `sleep`.
* Concurrency is designed to be data-race free (snapshot isolation + shared-nothing VMs), but run `go test -race ./...` on a machine with a C compiler to verify on other platforms.

## Project layout

```
.
├── ast.go           # Abstract syntax tree
├── builtins.go      # Native functions
├── code.go          # Opcodes
├── compiler.go      # AST to bytecode (with source maps)
├── concurrency.go   # Task spawning, snapshot isolation, shared output
├── diagnostic.go    # Error formatting
├── lexer.go         # Tokenizer
├── main.go          # CLI and embedded web server
├── object.go        # Runtime values
├── parser.go        # Pratt parser
├── token.go         # Token types and keywords
├── vm.go            # Bytecode interpreter
├── vm\\\_test.go       # Tests
├── examples/        # Sample .veri programs
└── web/
    └── index.html   # VeriCode Studio (Monaco)
```

\---

## Tests

```bash
go test ./...

# Concurrency is covered by the race detector on platforms with a C compiler:
go test -race ./...
```

\---

## Web IDE

```bash
./veri -web
```

Open [http://localhost:8080](http://localhost:8080).

* **Examples** dropdown — curated samples
* **Click the filename** to rename before save
* **Ctrl+Enter** — run
* **Ctrl+S** — download `.veri` file
* **Open** — load a file from disk

Programs run on the server via `POST /api/run`.

\---

## Language sketch (0.2)

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

loop item in \\\[1, 2, 3] {
  print item;
}

func greet(name) {
  print "Hi, ${name}";
}
greet("World");

let h = {"a": 1};
h\\\["b"] = 2;
print keys(h);

// concurrency
let ch = chan();
let t = spawn func() {
  send(ch, 42);
  return "done";
}();
print recv(ch);
print await t;
```

\---

## Contributing

Ideas, issues, and pull requests are welcome — especially for the sequential core and concurrency model (bugs, tests, examples) and early design discussion for **0.3 parallelism**.

Before proposing large features, open an issue so the design can stay consistent with the roadmap.

\---

## License

Add a license file before publishing (for example MIT). Until then, all rights reserved by the author.

\---

## Credits

Implemented in Go as a classic bytecode language: lexer, Pratt parser, compiler, and stack VM.
