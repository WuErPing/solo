// Command genzod generates TypeScript Zod schemas from Go structs annotated
// with a //genzod comment in the protocol package.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/WuErPing/solo/protocol/genzod"
)

func main() {
	var (
		input  = flag.String("input", ".", "Directory containing Go source files")
		output = flag.String("output", "", "Output TypeScript file path (single-file mode)")
		multi  = flag.String("multi", "", "Output directory for per-domain files + barrel (multi-file mode)")
	)
	flag.Parse()

	if *multi != "" {
		absInput, err := filepath.Abs(*input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve input: %v\n", err)
			os.Exit(1)
		}
		if err := genzod.GenerateMulti(absInput, *multi); err != nil {
			fmt.Fprintf(os.Stderr, "generate multi: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *output == "" {
		fmt.Fprintln(os.Stderr, "Usage: genzod -input <dir> (-output <file.ts> | -multi <outDir>)")
		os.Exit(1)
	}

	absInput, err := filepath.Abs(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve input: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output dir: %v\n", err)
		os.Exit(1)
	}

	out, err := os.Create(*output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create output file: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := out.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close output file: %v\n", err)
			os.Exit(1)
		}
	}()

	if err := genzod.Generate(absInput, out); err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
}
