package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// repl is a persistent session: each input line is compiled into the same
// global table, so variables and functions stay bound between lines.
type repl struct {
	vm      *VM
	symbols map[string]int
	args    []string
}

func newREPL(args []string) *repl {
	r := &repl{args: append([]string(nil), args...)}
	r.reset()
	return r
}

func (r *repl) reset() {
	vm := NewVM(&Bytecode{})
	vm.SetAllowIO(true)
	vm.SetArgs(r.args)
	r.vm = vm
	r.symbols = make(map[string]int)
}

// Eval compiles and runs one complete input chunk, returning the output it
// produced. The session keeps its globals whether or not the chunk succeeded.
func (r *repl) Eval(source string) (string, error) {
	lexer := NewLexer(source)
	parser := NewParser(lexer)
	parser.SetFilename("<repl>")
	program := parser.ParseProgram()
	if errs := parser.Errors(); len(errs) > 0 {
		return "", fmt.Errorf("%s", strings.Join(errs, "\n"))
	}

	compiler := NewCompilerWithSource("<repl>", source)
	compiler.SeedGlobals(r.symbols)
	compiler.SetREPLMode(true)
	if err := compiler.Compile(program); err != nil {
		return "", err
	}
	bc := compiler.Bytecode()
	for i, name := range bc.Symbols {
		if _, ok := r.symbols[name]; !ok {
			r.symbols[name] = i
		}
	}

	before := r.vm.out.Len()
	err := r.vm.RunBytecode(bc)
	out := r.vm.out.Since(before)
	return out, err
}

func runREPL(in io.Reader, out, errOut io.Writer, args []string) {
	newREPL(args).Run(in, out, errOut)
}

func (r *repl) Run(in io.Reader, out, errOut io.Writer) {
	fmt.Fprintf(out, "VeriCode %s — interactive session\n", version)
	fmt.Fprintln(out, "Type :help for commands, :quit to exit.")

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var buf strings.Builder

	for {
		if buf.Len() == 0 {
			fmt.Fprint(out, "veri> ")
		} else {
			fmt.Fprint(out, "...   ")
		}
		if !scanner.Scan() {
			fmt.Fprintln(out)
			return
		}
		line := scanner.Text()

		if buf.Len() == 0 {
			switch cmd := strings.TrimSpace(line); cmd {
			case "":
				continue
			case ":quit", ":q", ":exit":
				return
			case ":help", ":h":
				printREPLHelp(out)
				continue
			case ":reset":
				r.reset()
				fmt.Fprintln(out, "session reset")
				continue
			default:
				if strings.HasPrefix(cmd, ":") {
					fmt.Fprintf(errOut, "unknown command %q; try :help\n", cmd)
					continue
				}
			}
		}

		buf.WriteString(line)
		buf.WriteByte('\n')
		source := buf.String()
		if replIncomplete(source) {
			continue
		}
		buf.Reset()

		result, err := r.Eval(source)
		if result != "" {
			fmt.Fprint(out, result)
			if !strings.HasSuffix(result, "\n") {
				fmt.Fprintln(out)
			}
		}
		if err != nil {
			fmt.Fprintln(errOut, err.Error())
		}
	}
}

func printREPLHelp(out io.Writer) {
	fmt.Fprintln(out, "Commands:")
	fmt.Fprintln(out, "  :help    show this help")
	fmt.Fprintln(out, "  :reset   forget all variables and functions")
	fmt.Fprintln(out, "  :quit    leave the session")
	fmt.Fprintln(out, "Imports are not available in the REPL; run a file with veri -run instead.")
}

// replIncomplete reports whether more input is needed because braces are
// still open. Strings are single tokens, so braces inside them do not count.
func replIncomplete(source string) bool {
	depth := 0
	lexer := NewLexer(source)
	for {
		tok := lexer.NextToken()
		if tok.Type == EOF {
			break
		}
		switch tok.Type {
		case LBRACE:
			depth++
		case RBRACE:
			depth--
		}
	}
	return depth > 0
}
