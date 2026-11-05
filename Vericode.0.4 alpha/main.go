package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const version = "0.7"

//go:embed web/*
var embeddedWeb embed.FS

// resolveWebDir returns an http.FileSystem for the web/ assets.
// Strategy (in order):
//  1. If ./web exists on disk (dev mode), use it — so edits to index.html
//     show up without recompiling.
//  2. Otherwise use the assets embedded in the binary via //go:embed,
//     so the .exe works from any working directory with no dependencies.
func resolveWebDir() (http.FileSystem, string) {
	cwd, _ := os.Getwd()
	candidate := filepath.Join(cwd, "web")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return http.Dir(candidate), "disk: " + candidate
	}

	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		sidecar := filepath.Join(exeDir, "web")
		if info, err := os.Stat(sidecar); err == nil && info.IsDir() {
			return http.Dir(sidecar), "disk: " + sidecar
		}
	}

	sub, err := fs.Sub(embeddedWeb, "web")
	if err != nil {
		return nil, ""
	}
	return http.FS(sub), "embedded"
}

type CodeRequest struct {
	Code string `json:"code"`
}

type CodeResponse struct {
	Output string `json:"output,omitempty"`
	Errors string `json:"errors,omitempty"`
}

// compileCode lexes, parses, and compiles source without executing it.
func compileCode(code, filename string) (*Bytecode, string) {
	lexer := NewLexer(code)
	parser := NewParser(lexer)
	parser.SetFilename(filename)
	program := parser.ParseProgram()

	if errs := parser.Errors(); len(errs) > 0 {
		return nil, strings.Join(errs, "\n")
	}

	// Compile with source so diagnostics can show line + snippet
	compiler := NewCompilerWithSource(filename, code)
	if err := compiler.Compile(program); err != nil {
		return nil, err.Error()
	}
	return compiler.Bytecode(), ""
}

func runCode(code, filename string, allowIO bool, args []string) (string, string) {
	return runCodeLive(code, filename, allowIO, args, nil)
}

// runCodeLive runs a program and, when live is non-nil, streams each printed
// line to it immediately. The full output is still returned.
func runCodeLive(code, filename string, allowIO bool, args []string, live io.Writer) (string, string) {
	bytecode, errMsg := compileCode(code, filename)
	if errMsg != "" {
		return "", errMsg
	}

	// Execute. The web IDE disables file access (allowIO=false); the CLI
	// enables it for scripts that read or write CSV/JSON files.
	vm := NewVM(bytecode)
	vm.SetAllowIO(allowIO)
	vm.SetArgs(args)
	if live != nil {
		vm.SetLiveOutput(live)
	}
	if err := vm.Run(); err != nil {
		return "", err.Error()
	}

	out := vm.Output()
	if out == "" {
		out = "(no output)"
	}
	return out, ""
}

// readSourceFile reads a program and strips a UTF-8 BOM if present.
func readSourceFile(path string) (string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(string(src), "\xef\xbb\xbf"), nil
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(CodeResponse{Errors: "Invalid JSON body"})
		return
	}

	// Strip non-breaking spaces that Monaco can insert
	code := strings.ReplaceAll(req.Code, "\u00A0", " ")

	output, errMsg := runCode(code, "<input>", false, nil)
	if errMsg != "" {
		json.NewEncoder(w).Encode(CodeResponse{Errors: errMsg})
		return
	}
	json.NewEncoder(w).Encode(CodeResponse{Output: output})
}

func printUsage() {
	fmt.Printf("VeriCode %s\n\n", version)
	fmt.Println("Usage:")
	fmt.Println("  veri -run <file.veri> [args...]   run a source file (file I/O enabled)")
	fmt.Println("  veri -check <file.veri>           parse and compile without running")
	fmt.Println("  veri -e '<code>' [args...]        run inline source (file I/O enabled)")
	fmt.Println("  veri -repl [args...]              start an interactive session")
	fmt.Println("  veri -web                         start the web IDE on :8080 (file I/O disabled)")
	fmt.Println("  veri -version                     print the version")
}

func main() {
	webFlag := flag.Bool("web", false, "Start the VeriCode web IDE")
	runFlag := flag.String("run", "", "Run a .veri source file")
	checkFlag := flag.String("check", "", "Parse and compile a .veri file without running it")
	evalFlag := flag.String("e", "", "Run inline source")
	replFlag := flag.Bool("repl", false, "Start an interactive session")
	versionFlag := flag.Bool("version", false, "Print the version")
	flag.Usage = printUsage
	flag.Parse()

	switch {
	case *versionFlag:
		fmt.Println(version)

	case *runFlag != "":
		code, err := readSourceFile(*runFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot read file '%s': %v\n", *runFlag, err)
			os.Exit(1)
		}
		// Stream output so long-running programs (servers, loops) show progress
		// while they run instead of only at exit.
		_, errMsg := runCodeLive(code, *runFlag, true, flag.Args(), os.Stdout)
		if errMsg != "" {
			fmt.Fprintln(os.Stderr, errMsg)
			os.Exit(1)
		}

	case *checkFlag != "":
		code, err := readSourceFile(*checkFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot read file '%s': %v\n", *checkFlag, err)
			os.Exit(1)
		}
		if _, errMsg := compileCode(code, *checkFlag); errMsg != "" {
			fmt.Fprintln(os.Stderr, errMsg)
			os.Exit(1)
		}
		fmt.Printf("%s: ok\n", *checkFlag)

	case *evalFlag != "":
		_, errMsg := runCodeLive(*evalFlag, "<eval>", true, flag.Args(), os.Stdout)
		if errMsg != "" {
			fmt.Fprintln(os.Stderr, errMsg)
			os.Exit(1)
		}

	case *replFlag:
		runREPL(os.Stdin, os.Stdout, os.Stderr, flag.Args())

	case *webFlag:
		webFS, source := resolveWebDir()
		if webFS == nil {
			fmt.Fprintln(os.Stderr, "error: cannot find web assets (no ./web dir, no <exe>/web dir, and binary was built without embedded assets)")
			os.Exit(1)
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/api/run", handleRun)
		mux.Handle("/", http.FileServer(webFS))
		fmt.Println("VeriCode Studio → http://localhost:8080")
		fmt.Println("  assets:", source)
		if err := http.ListenAndServe(":8080", mux); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}

	default:
		printUsage()
	}
}
