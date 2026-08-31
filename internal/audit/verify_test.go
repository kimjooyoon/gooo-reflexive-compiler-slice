package audit

import "testing"

func TestIndependentReducerUsesFixedPrecedence(t *testing.T) {
	unknown := unknown{Stage: "stage", Step: "step", Reason: "reason", UnknownClass: "DIRECT_MISSING", NextOperation: "next", BlockedBy: []string{}}
	refutation := refutation{Stage: "stage", Step: "step", Reason: "reason", Counterexample: "counterexample"}
	if reduce(nil, nil) != "CLOSED" {
		t.Fatal("empty evidence must be CLOSED")
	}
	if reduce([]unknown{unknown}, nil) != "UNKNOWN" {
		t.Fatal("unknown evidence must be UNKNOWN")
	}
	if reduce([]unknown{unknown}, []refutation{refutation}) != "REFUTED" {
		t.Fatal("refutation must dominate UNKNOWN")
	}
}
