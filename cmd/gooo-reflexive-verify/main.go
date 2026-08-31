package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/gooo-reflexive-compiler-slice/internal/audit"
)

func main() {
	var options audit.Options
	flag.StringVar(&options.PhasePath, "phase", "", "phase graph")
	flag.StringVar(&options.SourcePath, "source", "", "original source Gooo")
	flag.StringVar(&options.BaselineDir, "baseline-dir", "", "first execution output")
	flag.StringVar(&options.CandidateDir, "candidate-dir", "", "re-execution output")
	flag.StringVar(&options.Expected, "expected", "", "expected decision")
	flag.StringVar(&options.Output, "output", "", "verification report")
	flag.Parse()

	report, err := audit.Verify(options)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("independent-verifier: decision=%s first=%s rerun=%s rollback=%t\n", report.Decision, report.FirstExecutionDigest, report.RerunExecutionDigest, report.Rollback.Possible)
}
