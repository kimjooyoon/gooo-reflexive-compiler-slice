package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/gooo-reflexive-compiler-slice/internal/compiler"
)

func main() {
	var options compiler.CompileOptions
	var inspectPhase bool
	var jsonOutput bool
	flag.StringVar(&options.PhasePath, "phase", "", "released-input Gooo phase graph")
	flag.StringVar(&options.InputPath, "input", "", "source Gooo or generated semantic IR")
	flag.StringVar(&options.InputKind, "input-kind", "source", "source or semantic-ir")
	flag.StringVar(&options.SourcePath, "source", "", "original source Gooo for lineage")
	flag.StringVar(&options.OutputDir, "output-dir", "", "output directory")
	flag.StringVar(&options.RunID, "run-id", "", "stable run identifier")
	flag.StringVar(&options.Role, "role", "", "baseline or candidate")
	flag.BoolVar(&inspectPhase, "inspect-phase", false, "validate and summarize a Gooo phase graph")
	flag.BoolVar(&jsonOutput, "json", false, "emit machine-readable inspection output")
	flag.Parse()
	if inspectPhase {
		summary, err := compiler.SummarizePhase(options.PhasePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if jsonOutput {
			if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			if !summary.Valid {
				os.Exit(3)
			}
			return
		}
		fmt.Printf("phase-graph: topology=%s activities=%d typed-edges=%d valid=%t localization-stages=%d\n", summary.Topology, summary.ActivityCount, summary.TypedEdgeCount, summary.Valid, summary.LocalizationStages)
		if !summary.Valid {
			os.Exit(3)
		}
		return
	}

	receipt, err := compiler.Compile(options)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Printf("reflexive-compiler: decision=%s role=%s input=%s execution=%s\n", receipt.Decision, receipt.Role, receipt.InputKind, receipt.ExecutionDigest)
}
