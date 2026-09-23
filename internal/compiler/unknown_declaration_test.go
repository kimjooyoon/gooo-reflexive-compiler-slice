package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPhaseGraphRetainsUnknownDeclaration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phase.gooo")
	data := []byte(`package reflexive
namespace reflexive
entity SourceGraph id "source"
activity NormalizeSource(SourceGraph) -> SemanticIR computes "reflexive.normalize:v1;input=GOOO"
activity EmitBackend(SemanticIR) -> GeneratedBackend computes "reflexive.backend-go:v1;authority=SOURCE_GOOO_GRAPH;artifact=BACKEND_ONLY"
activity VerifyReplay(SemanticIR) -> Evidence computes "reflexive.replay:v1;input=GENERATED_SEMANTIC_IR;rollback=RETAIN_BASELINE"
unknown_typo value
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	summary, err := SummarizePhase(path)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Valid || len(summary.GraphUnknowns) == 0 {
		t.Fatalf("unknown declaration was not retained: %#v", summary)
	}
}
