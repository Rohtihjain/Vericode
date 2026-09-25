# VeriCode

# 

# 0.1 alpha — a small, evolving programming language written in Go.

# 

# VeriCode starts as a clear, approachable language for learning and scripting. The long-term aim is to make concurrency, parallelism, and data-oriented work first-class and practical — so programs can grow from simple scripts into concurrent and data-heavy workloads without drowning in boilerplate.

# 

# 

# 

# Pipeline today: Lexer → Parser → Compiler → Bytecode VM

# Direction: sequential core → concurrent runtime → parallel data transforms → scalable data tools

# 

# 

# 

# Vision

# 

# Many languages support concurrency and large-scale data processing, but they often demand a steep jump in complexity. VeriCode’s goal is different in emphasis, not in denying what others already do well:

# 

# 

# 

# 

# 

# Concurrency that stays readable — tasks and communication as a natural part of the language

# 

# 

# 

# Parallelism for data work — parallel maps, pipelines, and batch transforms over collections

# 

# 

# 

# A path from small to serious — same language from print "hello" to concurrent and data-oriented programs

# 

# 

# 

# Safe defaults — design toward avoiding data races and surprising shared-state bugs

# 

# 0.1 is the foundation: a solid sequential language and tooling.

# Later phases add the concurrent and data layers on top of that foundation.

# 

# 

# 

# Current status (0.1 alpha)

# 

# Usable for demos, learning, and sequential scripts. Not production-hardened; concurrency / parallelism / big-data features are planned, not shipped yet.

# 

# What works today

# 

# 

# 

# 

# 

# 

# 

# Area

# 

# 

# 

# Support

# 

# 

# 

# 

# 

# Types

# 

# 

# 

# integers, floats (incl. scientific 1e3), booleans, strings, chars, arrays, hashes, null, functions

# 

# 

# 

# 

# 

# Control

# 

# 

# 

# if / else, loop while, loop n times, loop x in arr, break / continue

# 

# 

# 

# 

# 

# Functions

# 

# 

# 

# first-class, closures, recursion, named top-level func name(...) { }

# 

# 

# 

# 

# 

# Operators

# 

# 

# 

# arithmetic, comparison, logical, compound assign (+= …), pipe |>, index \& slice

# 

# 

# 

# 

# 

# Strings

# 

# 

# 

# escapes, ${expr} interpolation, format("{}", …)

# 

# 

# 

# 

# 

# Collections

# 

# 

# 

# arrays \& hashes, index read/write, slicing (arr\[1:3], str\[:2])

# 

# 

# 

# 

# 

# Stdlib

# 

# 

# 

# len, push/pop, map/filter/reduce, range, math (abs/min/max/sqrt/pow), string (split/join/trim/contains/replace/format)

# 

# 

# 

# 

# 

# Tooling

# 

# 

# 

# CLI runner, web IDE (Monaco), source-aware error messages

# 

# 

# 

# Roadmap

# 

# 

# 

# 

# 

# 

# 

# Phase

# 

# 

# 

# Theme

# 

# 

# 

# Direction

# 

# 

# 

# 

# 

# 0.1 (now)

# 

# 

# 

# Core language

# 

# 

# 

# Types, functions, slices, stdlib, CLI + web IDE

# 

# 

# 

# 

# 

# 0.2

# 

# 

# 

# Concurrency

# 

# 

# 

# Lightweight tasks, message passing / channels, clear spawn \& join model

# 

# 

# 

# 

# 

# 0.3

# 

# 

# 

# Parallelism

# 

# 

# 

# Parallel collection ops, worker pools, CPU-bound parallel loops

# 

# 

# 

# 

# 

# 0.4

# 

# 

# 

# Data-oriented

# 

# 

# 

# Pipelines, batch transforms, practical I/O (e.g. CSV/JSON), table-like structures

# 

# 

# 

# 

# 

# Later

# 

# 

# 

# Scale \& polish

# 

# 

# 

# Richer runtime, optional native hooks, tooling — only as the core stays solid

# 

# This roadmap is intentional: concurrency before “big data.” A trustworthy concurrent runtime is the base for parallel and data features.

# 

# 

# 

# Requirements

# 

# 

# 

# 

# 

# Go 1.22+

# 

# 

# 

# Build

# 

# git clone https://github.com/Rohtihjain/Vericode.git

# cd vericode

# go build -o veri .

# 

# Windows:

# 

# go build -o veri.exe .

# 

# 

# 

# Run

# 

# \# Run a source file

# ./veri -run examples/hello.veri

# 

# \# Web IDE → http://localhost:8080

# ./veri -web

# 

# \# Usage

# ./veri

# 

# Windows: use .\\veri.exe instead of ./veri.

# Source files conventionally use the .veri extension.

# 

# 

# 

# Quick examples (0.1)

# 

# Hello + interpolation

# 

# let name = "Ada";

# print "Hello, ${name}!";

# print format("score = {}", 95);

# 

# Named function + recursion

# 

# func fact(n) {

# &#x20; if n <= 1 { return 1; }

# &#x20; return n \* fact(n - 1);

# }

# print fact(5);

# 

# Slices + functional tools

# 

# let nums = range(0, 10);

# print nums\[2:5];              // \[2, 3, 4]

# print "hello"\[1:4];           // ell

# 

# print map(nums, func(n) { return n \* n; });

# print 5 |> func(x) { return x \* x; };

# 

# Floats + math

# 

# print 1e3;                    // 1000

# print 2.5e-1;                 // 0.25

# print sqrt(16.0);

# print pow(2, 10);

# 

# 

# 

# Project layout

# 

# .

# ├── ast.go           # Abstract syntax tree

# ├── builtins.go      # Native functions

# ├── code.go          # Opcodes

# ├── compiler.go      # AST → bytecode (+ source maps)

# ├── diagnostic.go    # Error formatting

# ├── lexer.go         # Tokenizer

# ├── main.go          # CLI + embedded web server

# ├── object.go        # Runtime values

# ├── parser.go        # Pratt parser

# ├── token.go         # Token types \& keywords

# ├── vm.go            # Bytecode interpreter

# ├── vm\_test.go       # Tests

# ├── examples/        # Sample .veri programs

# └── web/

# &#x20;   └── index.html   # VeriCode Studio (Monaco)

# 

# 

# 

# Tests

# 

# go test ./...

# 

# 

# 

# Web IDE

# 

# ./veri -web

# 

# Open http://localhost:8080.

# 

# 

# 

# 

# 

# Examples dropdown — curated samples

# 

# 

# 

# Click the filename to rename before save

# 

# 

# 

# Ctrl+Enter — run · Ctrl+S — download .veri

# 

# 

# 

# Open — load a file from disk

# 

# Programs run on the server via POST /api/run.

# 

# 

# 

# Language sketch (0.1)

# 

# let x = 10;

# x += 1;

# 

# if x > 5 {

# &#x20; print "big";

# } else {

# &#x20; print "small";

# }

# 

# loop while x > 0 {

# &#x20; x -= 1;

# }

# 

# loop item in \[1, 2, 3] {

# &#x20; print item;

# }

# 

# func greet(name) {

# &#x20; print "Hi, ${name}";

# }

# greet("World");

# 

# let h = {"a": 1};

# h\["b"] = 2;

# print keys(h);

# 

# 

# 

# Contributing

# 

# Ideas, issues, and PRs are welcome once the repo is public — especially around the sequential core (bugs, tests, examples) and early design discussion for 0.2 concurrency.

# 

# Before proposing large features, open an issue so the design can stay consistent with the roadmap.

# 

# 

# 

# License

# 

# Add a license file before publishing (e.g. MIT). Until then, all rights reserved by the author.

# 

# 

# 

# Credits

# 

# Implemented in Go as a classic bytecode language: lexer, Pratt parser, compiler, and stack VM.

