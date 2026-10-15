package main

import (
	"fmt"
	"strings"
)

type SourceLocation struct {
	Filename string
	Line     int
	Col      int
}

type StackFrameInfo struct {
	FnName   string
	Filename string
	Line     int
	Col      int
}

// FormatDiagnostic formats a Rust/Clang-style visual error with source line and caret.
// Uses plain text (no ANSI) so it works in both CLI and the web IDE.
func FormatDiagnostic(filename, source, errType, msg string, line, col int) string {
	if filename == "" {
		filename = "<input>"
	}
	if line < 1 {
		line = 1
	}
	if col < 1 {
		col = 1
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf("Error [%s]: %s\n", errType, msg))
	out.WriteString(fmt.Sprintf("  --> %s:%d:%d\n", filename, line, col))
	out.WriteString("   |\n")

	if source != "" {
		lines := strings.Split(source, "\n")
		if line-1 < len(lines) {
			rawLine := lines[line-1]
			cleanLine := strings.ReplaceAll(rawLine, "\t", "    ")
			lineNumStr := fmt.Sprintf("%d", line)
			padding := strings.Repeat(" ", len(lineNumStr))

			out.WriteString(fmt.Sprintf(" %s | %s\n", lineNumStr, cleanLine))

			caretIndent := col - 1
			if caretIndent < 0 {
				caretIndent = 0
			}
			if caretIndent > len(cleanLine) {
				caretIndent = len(cleanLine)
			}
			carets := strings.Repeat(" ", caretIndent) + "^"
			out.WriteString(fmt.Sprintf(" %s | %s %s\n", padding, carets, msg))
		}
	}
	out.WriteString("   |\n")
	return out.String()
}

// IsFormattedDiagnostic reports whether s is a full diagnostic block
// (header line "Error [...]: ..." plus a "-->" location) rather than a
// bare message.
func IsFormattedDiagnostic(s string) bool {
	return strings.HasPrefix(s, "Error [") && strings.Contains(s, "\n  --> ")
}

// DiagnosticHeadline returns the message from a diagnostic's first line,
// with the "Error [Type]: " prefix stripped. For bare messages it returns
// the input unchanged.
func DiagnosticHeadline(s string) string {
	first := s
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		first = s[:i]
	}
	if i := strings.Index(first, "]: "); i >= 0 {
		return first[i+3:]
	}
	return first
}

// FormatLoc renders a source location as "file:line:col" for inline mentions.
func FormatLoc(loc SourceLocation) string {
	name := loc.Filename
	if name == "" {
		name = "<input>"
	}
	if loc.Line < 1 {
		return name
	}
	return fmt.Sprintf("%s:%d:%d", name, loc.Line, loc.Col)
}

// FormatStackTrace formats the call stack trace (plain text). Frames are
// given innermost-first; they print in that order so the error site leads.
func FormatStackTrace(frames []StackFrameInfo) string {
	if len(frames) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString("Call Stack:\n")
	for i := 0; i < len(frames); i++ {
		f := frames[i]
		fnName := f.FnName
		if fnName == "" {
			fnName = "<anonymous>"
		}
		filename := f.Filename
		if filename == "" {
			filename = "<input>"
		}
		out.WriteString(fmt.Sprintf("  --> at %s (%s:%d:%d)\n", fnName, filename, f.Line, f.Col))
	}
	return out.String()
}

// LookupSourceMap returns the source location for a given instruction offset.
func LookupSourceMap(sourceMap []SourceLocation, pos int) SourceLocation {
	if len(sourceMap) == 0 || pos < 0 {
		return SourceLocation{}
	}
	if pos >= len(sourceMap) {
		pos = len(sourceMap) - 1
	}
	for i := pos; i >= 0; i-- {
		loc := sourceMap[i]
		if loc.Line > 0 {
			return loc
		}
	}
	return SourceLocation{}
}
