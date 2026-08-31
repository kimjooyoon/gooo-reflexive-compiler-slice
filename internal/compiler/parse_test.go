package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizationProgramIsDeclarative(t *testing.T) {
	program := `reflexive.normalize:v1;input=GOOO;normal_form=sort-by-stable-id;required_entity=gooo://required`
	options := parseProgram(program)
	if options["input"] != "GOOO" || options["normal_form"] != "sort-by-stable-id" || options["required_entity"] != "gooo://required" {
		t.Fatalf("parsed options = %#v", options)
	}
}

func TestReducePrecedence(t *testing.T) {
	unknown := Unknown{Stage: "stage", Step: "step", Reason: "reason", UnknownClass: "DIRECT_MISSING", NextOperation: "next", BlockedBy: []string{}}
	refutation := Refutation{Stage: "stage", Step: "step", Reason: "reason", Counterexample: "counterexample"}
	if got := reduce(nil, nil); got != DecisionClosed {
		t.Fatalf("closed reduction = %s", got)
	}
	if got := reduce([]Unknown{unknown}, nil); got != DecisionUnknown {
		t.Fatalf("unknown reduction = %s", got)
	}
	if got := reduce([]Unknown{unknown}, []Refutation{refutation}); got != DecisionRefuted {
		t.Fatalf("refuted reduction = %s", got)
	}
}

func TestKebabStableActivityID(t *testing.T) {
	if got := kebab("NormalizeSource"); got != "normalize-source" {
		t.Fatalf("kebab = %q", got)
	}
}

func TestPhaseGraphSupportsLegacyAndSplitTopologies(t *testing.T) {
	tests := []struct {
		name         string
		phase        string
		activities   int
		localization int
	}{
		{
			name: "legacy",
			phase: `package reflexive
namespace reflexive
entity SourceGraph id "source"
entity SemanticIR id "ir"
entity GeneratedBackend id "backend"
entity Evidence id "evidence"
activity NormalizeSource(SourceGraph) -> SemanticIR computes "reflexive.normalize:v1;input=GOOO;normal_form=sort-by-stable-id"
activity EmitBackend(SemanticIR) -> GeneratedBackend computes "reflexive.backend-go:v1;authority=SOURCE_GOOO_GRAPH;artifact=BACKEND_ONLY"
activity VerifyReplay(SemanticIR) -> Evidence computes "reflexive.replay:v1;input=GENERATED_SEMANTIC_IR;rollback=RETAIN_BASELINE"
`,
			activities: 3, localization: 1,
		},
		{
			name: "split",
			phase: `package reflexive
namespace reflexive
entity SourceGraph id "source"
entity ParsedSource id "parsed"
entity SemanticIR id "ir"
entity GeneratedBackend id "backend"
entity Evidence id "evidence"
topology reflexive.normalize.v2
precedence REFUTED > UNKNOWN > CLOSED
acceptance CLOSED UNKNOWN REFUTED
rollback RETAIN_BASELINE
authority SOURCE_GOOO_GRAPH
split NormalizeSource -> ParseSource + ValidateStableIDs
edge ParseSource -> ValidateStableIDs : ParsedSource
edge ValidateStableIDs -> EmitBackend : SemanticIR
edge ValidateStableIDs -> VerifyReplay : SemanticIR
activity ParseSource(SourceGraph) -> ParsedSource computes "reflexive.normalize:v1;input=GOOO;normal_form=sort-by-stable-id"
activity ValidateStableIDs(ParsedSource) -> SemanticIR computes "reflexive.validate-stable-ids:v1;input=PARSED_SOURCE;duplicate=REFUTED;missing=UNKNOWN"
activity EmitBackend(SemanticIR) -> GeneratedBackend computes "reflexive.backend-go:v1;authority=SOURCE_GOOO_GRAPH;artifact=BACKEND_ONLY"
activity VerifyReplay(SemanticIR) -> Evidence computes "reflexive.replay:v1;input=GENERATED_SEMANTIC_IR;rollback=RETAIN_BASELINE"
`,
			activities: 4, localization: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "phase.gooo")
			if err := os.WriteFile(path, []byte(test.phase), 0o644); err != nil {
				t.Fatal(err)
			}
			summary, err := SummarizePhase(path)
			if err != nil {
				t.Fatal(err)
			}
			if !summary.Valid || summary.ActivityCount != test.activities || summary.LocalizationStages != test.localization {
				t.Fatalf("summary = %#v", summary)
			}
		})
	}
}

func TestPhaseGraphReportsMissingEdgeAsUnknown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phase.gooo")
	phase := `package reflexive
namespace reflexive
topology reflexive.normalize.v2
authority SOURCE_GOOO_GRAPH
acceptance CLOSED UNKNOWN REFUTED
rollback RETAIN_BASELINE
activity ParseSource(SourceGraph) -> ParsedSource computes "reflexive.normalize:v1;input=GOOO;normal_form=sort-by-stable-id"
activity ValidateStableIDs(ParsedSource) -> SemanticIR computes "reflexive.validate-stable-ids:v1;input=PARSED_SOURCE"
activity EmitBackend(SemanticIR) -> GeneratedBackend computes "reflexive.backend-go:v1;authority=SOURCE_GOOO_GRAPH;artifact=BACKEND_ONLY"
activity VerifyReplay(SemanticIR) -> Evidence computes "reflexive.replay:v1;input=GENERATED_SEMANTIC_IR;rollback=RETAIN_BASELINE"
edge ParseSource -> ValidateStableIDs : ParsedSource
edge ValidateStableIDs -> EmitBackend : SemanticIR
`
	if err := os.WriteFile(path, []byte(phase), 0o644); err != nil {
		t.Fatal(err)
	}
	summary, err := SummarizePhase(path)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Valid || len(summary.GraphUnknowns) == 0 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestPhaseGraphReportsCycleAsRefuted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phase.gooo")
	phase := `package reflexive
namespace reflexive
topology reflexive.normalize.v2
precedence REFUTED > UNKNOWN > CLOSED
acceptance CLOSED UNKNOWN REFUTED
rollback RETAIN_BASELINE
authority SOURCE_GOOO_GRAPH
split NormalizeSource -> ParseSource + ValidateStableIDs
activity ParseSource(SourceGraph) -> ParsedSource computes "reflexive.normalize:v1;input=GOOO;normal_form=sort-by-stable-id"
activity ValidateStableIDs(ParsedSource) -> SemanticIR computes "reflexive.validate-stable-ids:v1;input=PARSED_SOURCE"
activity EmitBackend(SemanticIR) -> GeneratedBackend computes "reflexive.backend-go:v1;authority=SOURCE_GOOO_GRAPH;artifact=BACKEND_ONLY"
activity VerifyReplay(SemanticIR) -> Evidence computes "reflexive.replay:v1;input=GENERATED_SEMANTIC_IR;rollback=RETAIN_BASELINE"
edge ParseSource -> ValidateStableIDs : ParsedSource
edge ValidateStableIDs -> EmitBackend : SemanticIR
edge ValidateStableIDs -> VerifyReplay : SemanticIR
edge VerifyReplay -> ParseSource : SourceGraph
`
	if err := os.WriteFile(path, []byte(phase), 0o644); err != nil {
		t.Fatal(err)
	}
	summary, err := SummarizePhase(path)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Valid || len(summary.GraphRefutations) == 0 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestPhaseGraphReportsInvalidTypedEdgeAsRefuted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phase.gooo")
	phase := `package reflexive
namespace reflexive
topology reflexive.normalize.v2
precedence REFUTED > UNKNOWN > CLOSED
acceptance CLOSED UNKNOWN REFUTED
rollback RETAIN_BASELINE
authority SOURCE_GOOO_GRAPH
split NormalizeSource -> ParseSource + ValidateStableIDs
activity ParseSource(SourceGraph) -> ParsedSource computes "reflexive.normalize:v1;input=GOOO;normal_form=sort-by-stable-id"
activity ValidateStableIDs(ParsedSource) -> SemanticIR computes "reflexive.validate-stable-ids:v1;input=PARSED_SOURCE"
activity EmitBackend(SemanticIR) -> GeneratedBackend computes "reflexive.backend-go:v1;authority=SOURCE_GOOO_GRAPH;artifact=BACKEND_ONLY"
activity VerifyReplay(SemanticIR) -> Evidence computes "reflexive.replay:v1;input=GENERATED_SEMANTIC_IR;rollback=RETAIN_BASELINE"
edge ParseSource -> ValidateStableIDs : SemanticIR
edge ValidateStableIDs -> EmitBackend : SemanticIR
edge ValidateStableIDs -> VerifyReplay : SemanticIR
`
	if err := os.WriteFile(path, []byte(phase), 0o644); err != nil {
		t.Fatal(err)
	}
	summary, err := SummarizePhase(path)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Valid || len(summary.GraphRefutations) == 0 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestPhaseGraphReportsDuplicateRoleAsRefuted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phase.gooo")
	phase := `package reflexive
namespace reflexive
topology reflexive.normalize.v2
precedence REFUTED > UNKNOWN > CLOSED
acceptance CLOSED UNKNOWN REFUTED
rollback RETAIN_BASELINE
authority SOURCE_GOOO_GRAPH
split NormalizeSource -> ParseSource + ValidateStableIDs
activity ParseSource(SourceGraph) -> ParsedSource computes "reflexive.normalize:v1;input=GOOO;normal_form=sort-by-stable-id"
activity ParseSource(SourceGraph) -> ParsedSource computes "reflexive.normalize:v1;input=GOOO;normal_form=sort-by-stable-id"
activity ValidateStableIDs(ParsedSource) -> SemanticIR computes "reflexive.validate-stable-ids:v1;input=PARSED_SOURCE"
activity EmitBackend(SemanticIR) -> GeneratedBackend computes "reflexive.backend-go:v1;authority=SOURCE_GOOO_GRAPH;artifact=BACKEND_ONLY"
activity VerifyReplay(SemanticIR) -> Evidence computes "reflexive.replay:v1;input=GENERATED_SEMANTIC_IR;rollback=RETAIN_BASELINE"
edge ParseSource -> ValidateStableIDs : ParsedSource
edge ValidateStableIDs -> EmitBackend : SemanticIR
edge ValidateStableIDs -> VerifyReplay : SemanticIR
`
	if err := os.WriteFile(path, []byte(phase), 0o644); err != nil {
		t.Fatal(err)
	}
	summary, err := SummarizePhase(path)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Valid || len(summary.GraphRefutations) == 0 {
		t.Fatalf("summary = %#v", summary)
	}
}
