package compiler

import "testing"

func TestTerminalRecordPreservesPrecedenceAndFrontier(t *testing.T) {
	phase := Phase{Edges: []Edge{
		{From: "ValidateStableIDs", To: "EmitBackend", ValueType: "SemanticIR"},
		{From: "ValidateStableIDs", To: "VerifyReplay", ValueType: "SemanticIR"},
	}}
	unknown := Unknown{
		Stage: "NORMALIZE", Step: "REQUIRE_DECLARED_ENTITY", Reason: "REQUIRED_ENTITY_MISSING",
		UnknownClass: "DIRECT_MISSING", NextOperation: "ADD_REQUIRED_ENTITY", BlockedBy: []string{"required"},
	}
	refutation := Refutation{Stage: "NORMALIZE", Step: "CHECK_STABLE_ID_UNIQUENESS", Reason: "DUPLICATE_STABLE_ID", Counterexample: "duplicate"}
	record := buildTerminalRecord(phase, DecisionRefuted, []Unknown{unknown}, []Refutation{refutation})
	if record.Decision != DecisionRefuted || record.Stage != refutation.Stage || record.Reason != refutation.Reason {
		t.Fatalf("terminal record = %#v", record)
	}
	if record.CounterexampleDigest == "" || len(record.MinimalFrontier) != 2 || record.CauseEdge != record.MinimalFrontier[0] {
		t.Fatalf("terminal frontier = %#v", record)
	}

	unknownRecord := buildTerminalRecord(phase, DecisionUnknown, []Unknown{unknown}, nil)
	if unknownRecord.UnknownClass == "" || unknownRecord.NextOperation == "" || unknownRecord.BlockedBy == nil {
		t.Fatalf("UNKNOWN explanation = %#v", unknownRecord)
	}
}

func TestClosedTerminalRecordIsExplanationCarrying(t *testing.T) {
	record := buildTerminalRecord(Phase{}, DecisionClosed, nil, nil)
	if record.Stage == "" || record.Step == "" || record.Reason == "" || record.NextOperation == "" || record.CounterexampleDigest == "" {
		t.Fatalf("CLOSED explanation = %#v", record)
	}
	if record.BlockedBy == nil || record.MinimalFrontier == nil {
		t.Fatalf("CLOSED arrays must be present = %#v", record)
	}
}
