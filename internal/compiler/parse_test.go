package compiler

import "testing"

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
