package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

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

func runCode(code string) (string, string) {
	// Lex
	lexer := NewLexer(code)
	parser := NewParser(lexer)
	program := parser.ParseProgram()

	if errs := parser.Errors(); len(errs) > 0 {
		return "", strings.Join(errs, "\n")
	}

	// Compile with source so diagnostics can show line + snippet
	compiler := NewCompilerWithSource("<input>", code)
	if err := compiler.Compile(program); err != nil {
		return "", err.Error()
	}

	// Execute
	vm := NewVM(compiler.Bytecode())
	if err := vm.Run(); err != nil {
		return "", err.Error()
	}

	out := vm.Output()
	if out == "" {
		out = "(no output)"
	}
	return out, ""
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

	output, errMsg := runCode(code)
	if errMsg != "" {
		json.NewEncoder(w).Encode(CodeResponse{Errors: errMsg})
		return
	}
	json.NewEncoder(w).Encode(CodeResponse{Output: output})
}

func main() {
	webFlag := flag.Bool("web", false, "Start the VeriCode web IDE")
	runFlag := flag.String("run", "", "Run a .veri source file")
	flag.Parse()

	switch {
	case *runFlag != "":
		src, err := os.ReadFile(*runFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot read file '%s': %v\n", *runFlag, err)
			os.Exit(1)
		}
		// Strip UTF-8 BOM if present
		code := string(src)
		code = strings.TrimPrefix(code, "\xef\xbb\xbf")
		output, errMsg := runCode(code)
		if errMsg != "" {
			fmt.Fprintln(os.Stderr, errMsg)
			os.Exit(1)
		}
		fmt.Println(output)

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
		fmt.Println("VeriCode 0.2 alpha")
		fmt.Println("  veri -run <file.veri>   run a source file")
		fmt.Println("  veri -web               start the web IDE on :8080")
	}
}
