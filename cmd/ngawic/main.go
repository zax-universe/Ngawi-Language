package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ngawi/internal/codegen"
	"ngawi/internal/diag"
	"ngawi/internal/parser"
	"ngawi/internal/sema"
)

func printUsage() {
	fmt.Println("Usage: ngawic build <input.ngawi> [-o output] [-S]")
}

func hasNgawiExt(path string) bool {
	return strings.HasSuffix(path, ".ngawi")
}

func defaultOutputFromInput(input string) string {
	ext := filepath.Ext(input)
	if ext != "" {
		return input[:len(input)-len(ext)]
	}
	return input
}

func readFileAll(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// isImportLine returns true if the line is purely an import statement.
func isImportLine(line string) bool {
	s := strings.TrimLeft(line, " \t\r")
	if !strings.HasPrefix(s, "import") {
		return false
	}
	s = s[len("import"):]
	s = strings.TrimLeft(s, " \t")
	if !strings.HasPrefix(s, "\"") {
		return false
	}
	s = s[1:]
	idx := strings.Index(s, "\"")
	if idx < 0 {
		return false
	}
	s = s[idx+1:]
	s = strings.TrimLeft(s, " \t")
	if !strings.HasPrefix(s, ";") {
		return false
	}
	s = s[1:]
	s = strings.TrimLeft(s, " \t\r")
	return s == ""
}

type loadState struct {
	stack  []string // files being loaded (for cycle detection)
	loaded []string // files already fully loaded
	out    strings.Builder
}

func (ls *loadState) stackContains(canon string) int {
	for i, s := range ls.stack {
		if s == canon {
			return i
		}
	}
	return -1
}

func (ls *loadState) loadedContains(canon string) bool {
	for _, s := range ls.loaded {
		if s == canon {
			return true
		}
	}
	return false
}

func (ls *loadState) loadRecursive(path, importerFile string, importerLine int, targetText string) error {
	canon, err := filepath.Abs(path)
	if err != nil {
		diag.Error(path, 1, 1, "cannot resolve module path: %v", err)
		if importerFile != "" {
			diag.Note(importerFile, importerLine, 1, "while importing \"%s\"", targetText)
		}
		return err
	}

	if ls.loadedContains(canon) {
		return nil
	}

	if cycleIdx := ls.stackContains(canon); cycleIdx >= 0 {
		// Build cycle string
		parts := ls.stack[cycleIdx:]
		chain := strings.Join(parts, " -> ") + " -> " + canon
		diag.Error(canon, 1, 1, "import cycle detected: %s", chain)
		if importerFile != "" {
			diag.Note(importerFile, importerLine, 1, "while importing \"%s\"", targetText)
		}
		return fmt.Errorf("import cycle")
	}

	ls.stack = append(ls.stack, canon)

	source, err := readFileAll(canon)
	if err != nil {
		diag.Error(canon, 1, 1, "cannot read module: %v", err)
		if importerFile != "" {
			diag.Note(importerFile, importerLine, 1, "while importing \"%s\"", targetText)
		}
		ls.stack = ls.stack[:len(ls.stack)-1]
		return err
	}

	moduleProg, hadErr := parser.ParseProgram(canon, source)
	if hadErr {
		ls.stack = ls.stack[:len(ls.stack)-1]
		return fmt.Errorf("parse error in %s", canon)
	}

	dir := filepath.Dir(canon)
	for _, imp := range moduleProg.Imports {
		depPath := filepath.Join(dir, imp.Path)
		if !hasNgawiExt(depPath) {
			diag.ErrorSource(canon, source, imp.Line, imp.Col,
				"import path must end with .ngawi: %s", imp.Path)
			ls.stack = ls.stack[:len(ls.stack)-1]
			return fmt.Errorf("bad import path")
		}
		if err := ls.loadRecursive(depPath, canon, imp.Line, imp.Path); err != nil {
			ls.stack = ls.stack[:len(ls.stack)-1]
			return err
		}
	}

	// Append non-import lines from this file
	for _, line := range strings.Split(source, "\n") {
		if !isImportLine(line) {
			ls.out.WriteString(line)
			ls.out.WriteByte('\n')
		}
	}
	// Ensure trailing newline separation
	if ls.out.Len() > 0 {
		s := ls.out.String()
		if s[len(s)-1] != '\n' {
			ls.out.WriteByte('\n')
		}
	}

	// Pop from stack and mark as loaded
	ls.stack = ls.stack[:len(ls.stack)-1]
	ls.loaded = append(ls.loaded, canon)
	return nil
}

func loadSourceWithImports(input string) (string, error) {
	ls := &loadState{}
	if err := ls.loadRecursive(input, "", 0, ""); err != nil {
		return "", err
	}
	return ls.out.String(), nil
}

func main() {
	if len(os.Args) < 3 || os.Args[1] != "build" {
		printUsage()
		os.Exit(1)
	}

	input := os.Args[2]
	output := ""
	emitCOnly := false

	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-o":
			if i+1 >= len(os.Args) {
				diag.Error("<cli>", 1, 1, "missing value for -o")
				os.Exit(1)
			}
			i++
			output = os.Args[i]
		case "-S":
			emitCOnly = true
		default:
			diag.Error("<cli>", 1, 1, "unknown flag: %s", os.Args[i])
			os.Exit(1)
		}
	}

	if !hasNgawiExt(input) {
		diag.Error(input, 1, 1, "input file must have .ngawi extension")
		os.Exit(1)
	}

	source, err := loadSourceWithImports(input)
	if err != nil {
		os.Exit(1)
	}

	prog, hadErr := parser.ParseProgram(input, source)
	if hadErr {
		os.Exit(1)
	}

	if sema.CheckProgram(input, source, prog) {
		os.Exit(1)
	}

	if output == "" {
		output = defaultOutputFromInput(input)
	}

	cPath := output + ".c"
	if err := codegen.EmitC(input, prog, cPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if emitCOnly {
		fmt.Printf("C11 output: %s\n", cPath)
		return
	}

	// Find the runtime directory relative to the binary or use env
	runtimeDir := findRuntimeDir()
	cmd := fmt.Sprintf("gcc -std=c11 -O2 -I%s %s %s/ngawi_runtime.c -o %s",
		runtimeDir, cPath, runtimeDir, output)

	if rc := runShell(cmd); rc != 0 {
		diag.Error(input, 1, 1, "gcc failed (exit code %d)", rc)
		os.Exit(1)
	}

	fmt.Printf("Built native binary: %s\n", output)
}

func findRuntimeDir() string {
	// Check NGAWI_RUNTIME env
	if d := os.Getenv("NGAWI_RUNTIME"); d != "" {
		return d
	}
	// Default: src/runtime relative to cwd
	return "src/runtime"
}

func runShell(cmd string) int {
	// Use os/exec to run the command via sh
	proc := newShellCmd(cmd)
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	if err := proc.Run(); err != nil {
		if exitErr, ok := err.(interface{ ExitCode() int }); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}
