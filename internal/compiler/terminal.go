package compiler

import (
	"encoding/json"
	"strings"
)

const terminalSchema = "gooo/reflexive-terminal-record/v1"

func buildTerminalRecord(phase Phase, decision string, unknowns []Unknown, refutations []Refutation) TerminalRecord {
	record := TerminalRecord{
		Schema:          terminalSchema,
		Decision:        decision,
		UnknownClass:    "",
		BlockedBy:       []string{},
		MinimalFrontier: []FrontierEdge{},
		Counterexample:  "",
	}

	switch decision {
	case DecisionRefuted:
		if len(refutations) > 0 {
			refutation := refutations[0]
			record.Stage = refutation.Stage
			record.Step = refutation.Step
			record.Reason = refutation.Reason
			record.Counterexample = refutation.Counterexample
			record.CounterexampleDigest = digestEvidence(refutation)
			record.MinimalFrontier = frontierFor(phase, refutation.Stage, refutation.Step, refutation.Counterexample)
		}
	case DecisionUnknown:
		if len(unknowns) > 0 {
			unknown := unknowns[0]
			record.Stage = unknown.Stage
			record.Step = unknown.Step
			record.Reason = unknown.Reason
			record.UnknownClass = unknown.UnknownClass
			record.NextOperation = unknown.NextOperation
			record.BlockedBy = append([]string{}, unknown.BlockedBy...)
			record.CounterexampleDigest = digestEvidence(unknown)
			record.MinimalFrontier = frontierFor(phase, unknown.Stage, unknown.Step, strings.Join(unknown.BlockedBy, ","))
		}
	default:
		record.Stage = "TERMINAL"
		record.Step = "CLOSE_AFTER_VALIDATION"
		record.Reason = "NO_UNKNOWN_OR_REFUTATION"
		record.NextOperation = "RETAIN_RESULT"
		record.CounterexampleDigest = digestEvidence(struct {
			Decision string `json:"decision"`
			Stage    string `json:"stage"`
			Step     string `json:"step"`
			Reason   string `json:"reason"`
		}{Decision: decision, Stage: record.Stage, Step: record.Step, Reason: record.Reason})
	}
	if len(record.MinimalFrontier) > 0 {
		record.CauseEdge = record.MinimalFrontier[0]
	}
	return record
}

func digestEvidence(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "sha256:"
	}
	return digestBytes(data)
}

func frontierFor(phase Phase, stage, step, blocked string) []FrontierEdge {
	if edge, ok := parseFrontierEdge(blocked); ok {
		return []FrontierEdge{edge}
	}

	roles := []string{}
	switch {
	case stage == "SOURCE_READ":
		roles = []string{"ParseSource", "NormalizeSource"}
	case stage == "NORMALIZE":
		roles = []string{"ValidateStableIDs", "NormalizeSource", "ParseSource"}
	case stage == "SEMANTIC_IR_READ":
		roles = []string{"VerifyReplay", "EmitBackend"}
	case stage == "PHASE_GRAPH":
		roles = []string{"ParseSource", "ValidateStableIDs", "EmitBackend", "VerifyReplay", "NormalizeSource"}
	}
	for _, role := range roles {
		frontier := outgoingFrontier(phase, role)
		if len(frontier) > 0 {
			return frontier
		}
		frontier = incomingFrontier(phase, role)
		if len(frontier) > 0 {
			return frontier
		}
	}
	return []FrontierEdge{}
}

func outgoingFrontier(phase Phase, role string) []FrontierEdge {
	frontier := []FrontierEdge{}
	for _, edge := range phase.Edges {
		if edge.From == role {
			frontier = append(frontier, FrontierEdge{From: edge.From, To: edge.To, ValueType: edge.ValueType})
		}
	}
	return frontier
}

func incomingFrontier(phase Phase, role string) []FrontierEdge {
	frontier := []FrontierEdge{}
	for _, edge := range phase.Edges {
		if edge.To == role {
			frontier = append(frontier, FrontierEdge{From: edge.From, To: edge.To, ValueType: edge.ValueType})
		}
	}
	return frontier
}

func parseFrontierEdge(value string) (FrontierEdge, bool) {
	value = strings.TrimSpace(value)
	colon := strings.LastIndexByte(value, ':')
	if colon < 0 {
		return FrontierEdge{}, false
	}
	roles := strings.Fields(strings.TrimSpace(value[:colon]))
	valueType := strings.TrimSpace(value[colon+1:])
	if len(roles) != 3 || roles[1] != "->" || roles[0] == "" || roles[2] == "" || valueType == "" {
		return FrontierEdge{}, false
	}
	return FrontierEdge{From: roles[0], To: roles[2], ValueType: valueType}, true
}
