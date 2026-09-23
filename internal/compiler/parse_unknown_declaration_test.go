package compiler

import (
	"path/filepath"
	"testing"
)

func TestPhaseParserRefutesUnknownDeclaration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phase.gooo")
	phase := `package reflexive
namespace reflexive
entity SourceGraph id "source"
entity SemanticIR id "ir"
entity GeneratedBackend id "backend"
entity Evidence id "evidence"
activity NormalizeSource(SourceGraph) -> SemanticIR computes "reflexive.normalize:v1;input=GOOO;normal_form=sort-by-stable-id"
activity EmitBackend(SemanticIR) -> GeneratedBackend computes "reflexive.backend-go:v1;authority=SOURCE_GOOO_GRAPH;artifact=BACKEND_ONLY"
activity VerifyReplay(SemanticIR) -> Evidence computes "reflexive.replay:v1;input=GENERATED_SEMANTIC_IR;rollback=RETAIN_BASELINE"
unsupported-phase-declaration value
`
	if err := writeTestFile(path, phase); err != nil {
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
