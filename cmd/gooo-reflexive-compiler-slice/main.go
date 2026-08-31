package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/gooo-reflexive-compiler-slice/internal/compiler"
)

func main() {
	var options compiler.CompileOptions
	flag.StringVar(&options.PhasePath, "phase", "", "released-input Gooo phase graph")
	flag.StringVar(&options.InputPath, "input", "", "source Gooo or generated semantic IR")
	flag.StringVar(&options.InputKind, "input-kind", "source", "source or semantic-ir")
	flag.StringVar(&options.SourcePath, "source", "", "original source Gooo for lineage")
	flag.StringVar(&options.OutputDir, "output-dir", "", "output directory")
	flag.StringVar(&options.RunID, "run-id", "", "stable run identifier")
	flag.StringVar(&options.Role, "role", "", "baseline or candidate")
	flag.Parse()

	receipt, err := compiler.Compile(options)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Printf("reflexive-compiler: decision=%s role=%s input=%s execution=%s\n", receipt.Decision, receipt.Role, receipt.InputKind, receipt.ExecutionDigest)
}
