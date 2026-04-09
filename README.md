<div align="center">

```
\u2588\u2588\u2588\u2557   \u2588\u2588\u2557 \u2588\u2588\u2588\u2588\u2588\u2588\u2557  \u2588\u2588\u2588\u2588\u2588\u2557 \u2588\u2588\u2557    \u2588\u2588\u2557\u2588\u2588\u2557
\u2588\u2588\u2588\u2588\u2557  \u2588\u2588\u2551\u2588\u2588\u2554\u2550\u2550\u2550\u2550\u255d \u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2557\u2588\u2588\u2551    \u2588\u2588\u2551\u2588\u2588\u2551
\u2588\u2588\u2554\u2588\u2588\u2557 \u2588\u2588\u2551\u2588\u2588\u2551  \u2588\u2588\u2588\u2557\u2588\u2588\u2588\u2588\u2588\u2588\u2588\u2551\u2588\u2588\u2551 \u2588\u2557 \u2588\u2588\u2551\u2588\u2588\u2551
\u2588\u2588\u2551\u255a\u2588\u2588\u2557\u2588\u2588\u2551\u2588\u2588\u2551   \u2588\u2588\u2551\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2551\u2588\u2588\u2551\u2588\u2588\u2588\u2557\u2588\u2588\u2551\u2588\u2588\u2551
\u2588\u2588\u2551 \u255a\u2588\u2588\u2588\u2588\u2551\u255a\u2588\u2588\u2588\u2588\u2588\u2588\u2554\u255d\u2588\u2588\u2551  \u2588\u2588\u2551\u255a\u2588\u2588\u2588\u2554\u2588\u2588\u2588\u2554\u255d\u2588\u2588\u2551
\u255a\u2550\u255d  \u255a\u2550\u2550\u2550\u255d \u255a\u2550\u2550\u2550\u2550\u2550\u255d \u255a\u2550\u255d  \u255a\u2550\u255d \u255a\u2550\u2550\u255d\u255a\u2550\u2550\u255d \u255a\u2550\u255d
```

**Ngawi Programming Language \u2014 Go Edition**

*Write like Python. Run like C.*

![Language](https://img.shields.io/badge/language-Go-00ADD8?style=flat-square&logo=go)
![Status](https://img.shields.io/badge/status-experimental-orange?style=flat-square)
![Output](https://img.shields.io/badge/target-C11%20%2F%20Native-blue?style=flat-square)
![Author](https://img.shields.io/badge/by-azxm-ff69b4?style=flat-square)

</div>

---

## \ud83d\udcd6 Tentang Ngawi

**Ngawi** adalah bahasa pemrograman eksperimental yang dikompilasi ke C11 native binary. Compiler ini ditulis ulang dari C ke **Go** dengan arsitektur yang bersih dan modular.

Pipeline kompilasi:

```
Source (.ngawi) \u2192 Lexer \u2192 Parser (AST) \u2192 Sema \u2192 C11 Codegen \u2192 GCC \u2192 Native Binary
```

Ngawi punya dua "kepribadian" \u2014 keyword standar untuk yang suka clean, dan keyword alias lokal buat yang suka gaya sendiri.

---

## \ud83d\uddc2\ufe0f Struktur Proyek

```
ngawi-go/
\u2502
\u251c\u2500\u2500 \ud83d\udcc4 go.mod                         # Go module definition
\u251c\u2500\u2500 \ud83d\udcc4 README.md                      # Dokumentasi ini
\u2502
\u251c\u2500\u2500 \ud83d\udcc1 cmd/
\u2502   \u2514\u2500\u2500 \ud83d\udcc1 ngawic/
\u2502       \u251c\u2500\u2500 main.go                   # CLI entry point (build command, flags)
\u2502       \u2514\u2500\u2500 shell.go                  # Shell exec helper (GCC runner)
\u2502
\u251c\u2500\u2500 \ud83d\udcc1 internal/
\u2502   \u251c\u2500\u2500 \ud83d\udcc1 lexer/
\u2502   \u2502   \u251c\u2500\u2500 token.go                  # Token kinds & Token struct
\u2502   \u2502   \u2514\u2500\u2500 lexer.go                  # Tokenizer / scanner
\u2502   \u2502
\u2502   \u251c\u2500\u2500 \ud83d\udcc1 parser/
\u2502   \u2502   \u251c\u2500\u2500 ast.go                    # AST node types (Expr, Stmt, Program)
\u2502   \u2502   \u2514\u2500\u2500 parser.go                 # Recursive descent parser
\u2502   \u2502
\u2502   \u251c\u2500\u2500 \ud83d\udcc1 sema/
\u2502   \u2502   \u2514\u2500\u2500 sema.go                   # Semantic analysis & type checker
\u2502   \u2502
\u2502   \u251c\u2500\u2500 \ud83d\udcc1 codegen/
\u2502   \u2502   \u2514\u2500\u2500 codegen.go                # C11 code generator
\u2502   \u2502
\u2502   \u2514\u2500\u2500 \ud83d\udcc1 diag/
\u2502       \u2514\u2500\u2500 diag.go                   # Diagnostics (error/note + source snippet)
\u2502
\u251c\u2500\u2500 \ud83d\udcc1 runtime/
\u2502   \u251c\u2500\u2500 ngawi_runtime.h               # Runtime header (C)
\u2502   \u2514\u2500\u2500 ngawi_runtime.c               # Runtime implementation (C)
\u2502
\u2514\u2500\u2500 \ud83d\udcc1 examples/
    \u251c\u2500\u2500 hello.ngawi
    \u251c\u2500\u2500 factorial.ngawi
    \u251c\u2500\u2500 array_int.ngawi
    \u251c\u2500\u2500 string_builtins.ngawi
    \u251c\u2500\u2500 match.ngawi
    \u2514\u2500\u2500 ...
```

---

## \u2699\ufe0f Build & Install

### Prerequisites

- **Go** 1.21+
- **GCC** (untuk compile ke native binary)

### Compile Compiler

```bash
# Clone / extract project
cd ngawi-go

# Build binary compiler
go build -o ngawic ./cmd/ngawic

# Cek versi
./ngawic
# Usage: ngawic build <input.ngawi> [-o output] [-S]
```

---

## \ud83d\ude80 Cara Penggunaan

### Compile ke native binary

```bash
./