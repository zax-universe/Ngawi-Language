package diag

import (
	"fmt"
	"os"
	"strings"
)

func printSourceSnippet(source string, targetLine, targetCol int) {
	if source == "" || targetLine <= 0 || targetCol <= 0 {
		return
	}
	lines := strings.Split(source, "\n")
	if targetLine > len(lines) {
		return
	}
	line := lines[targetLine-1]
	fmt.Fprintln(os.Stderr, line)
	pad := ""
	for i := 1; i < targetCol; i++ {
		if i-1 < len(line) && line[i-1] == '\t' {
			pad += "\t"
		} else {
			pad += " "
		}
	}
	fmt.Fprintln(os.Stderr, pad+"^")
}

// Error prints an error diagnostic.
func Error(file string, line, col int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if file == "" {
		file = "<unknown>"
	}
	fmt.Fprintf(os.Stderr, "%s:%d:%d: error: %s\n", file, line, col, msg)
}

// Note prints a note diagnostic.
func Note(file string, line, col int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if file == "" {
		file = "<unknown>"
	}
	fmt.Fprintf(os.Stderr, "%s:%d:%d: note: %s\n", file, line, col, msg)
}

// ErrorSource prints an error diagnostic with a source snippet.
func ErrorSource(file, source string, line, col int, format string, args ...interface{}) {
	Error(file, line, col, format, args...)
	printSourceSnippet(source, line, col)
}

// NoteSource prints a note diagnostic with a source snippet.
func NoteSource(file, source string, line, col int, format string, args ...interface{}) {
	Note(file, line, col, format, args...)
	printSourceSnippet(source, line, col)
}
