package compiler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type CompileOptions struct {
	PhasePath  string
	InputPath  string
	InputKind  string
	SourcePath string
	OutputDir  string
	RunID      string
	Role       string
}

type executionProjection struct {
	Decision    string
	Unknowns    []Unknown
	Refutations []Refutation
	SemanticIR  string
	Generated   string
	Terminal    string
}

func Compile(options CompileOptions) (Receipt, error) {
	if options.InputKind != "source" && options.InputKind != "semantic-ir" {
		return Receipt{}, fmt.Errorf("input-kind must be source or semantic-ir")
	}
	if options.Role != "baseline" && options.Role != "candidate" {
		return Receipt{}, fmt.Errorf("role must be baseline or candidate")
	}
	if options.RunID == "" || options.OutputDir == "" || options.PhasePath == "" || options.InputPath == "" || options.SourcePath == "" {
		return Receipt{}, fmt.Errorf("phase, input, source, output-dir, and run-id are required")
	}
	phasePath, err := filepath.Abs(options.PhasePath)
	if err != nil {
		return Receipt{}, err
	}
	inputPath, err := filepath.Abs(options.InputPath)
	if err != nil {
		return Receipt{}, err
	}
	sourcePath, err := filepath.Abs(options.SourcePath)
	if err != nil {
		return Receipt{}, err
	}
	outputDir, err := filepath.Abs(options.OutputDir)
	if err != nil {
		return Receipt{}, err
	}
	phase, err := parsePhase(phasePath)
	if err != nil {
		return Receipt{}, err
	}
	sourceData, sourceDigest, err := readBytes(sourcePath)
	if err != nil {
		return Receipt{}, err
	}
	inputData, inputDigest, err := readBytes(inputPath)
	if err != nil {
		return Receipt{}, err
	}

	unknowns := append([]Unknown{}, phase.GraphUnknowns...)
	refutations := append([]Refutation{}, phase.GraphRefutations...)
	namespace := "unknown"
	declarations := []Declaration{}
	replayEvidence := false
	if options.InputKind == "source" {
		namespace, declarations, err = parseSource(inputData)
		if err != nil {
			unknowns = append(unknowns, Unknown{
				Stage: "SOURCE_READ", Step: "PARSE_GOOO_INPUT", Reason: "SOURCE_NOT_PARSEABLE",
				UnknownClass: sourceUnknownClass(err), NextOperation: "REPAIR_GOOO_INPUT", BlockedBy: []string{"SOURCE_BYTES"},
			})
		}
	} else {
		ir, _, _, loadErr := loadIR(inputPath)
		if loadErr != nil {
			unknowns = append(unknowns, Unknown{
				Stage: "SEMANTIC_IR_READ", Step: "READ_GENERATED_SEMANTIC_IR", Reason: "GENERATED_IR_MISSING_OR_MALFORMED",
				UnknownClass: "DIRECT_MISSING", NextOperation: "REGENERATE_SEMANTIC_IR", BlockedBy: []string{"SEMANTIC_IR_BYTES"},
			})
		} else {
			namespace, declarations = ir.Namespace, append([]Declaration(nil), ir.Declarations...)
			unknowns = append([]Unknown{}, ir.Evidence.Unknowns...)
			refutations = append([]Refutation{}, ir.Evidence.Refutations...)
			replayEvidence = true
			if ir.PhaseDigest != phase.Digest {
				unknowns = append(unknowns, Unknown{
					Stage: "SEMANTIC_IR_READ", Step: "VERIFY_PHASE_DIGEST", Reason: "SEMANTIC_IR_PHASE_STALE",
					UnknownClass: "STALE", NextOperation: "REGENERATE_SEMANTIC_IR", BlockedBy: []string{"PHASE_DIGEST"},
				})
			}
			if ir.OriginSourceDigest != sourceDigest {
				unknowns = append(unknowns, Unknown{
					Stage: "SEMANTIC_IR_READ", Step: "VERIFY_SOURCE_LINEAGE", Reason: "SEMANTIC_IR_SOURCE_MISMATCH",
					UnknownClass: "CONTRADICTORY_INPUT", NextOperation: "REBUILD_FROM_SOURCE_GOOO", BlockedBy: []string{"SOURCE_DIGEST"},
				})
			}
		}
	}

	if !replayEvidence {
		seen := map[string]bool{}
		for _, declaration := range declarations {
			if declaration.StableID == "" || declaration.Name == "" {
				unknowns = append(unknowns, Unknown{
					Stage: "NORMALIZE", Step: "BIND_STABLE_ID", Reason: "DECLARATION_ID_MISSING",
					UnknownClass: "DIRECT_MISSING", NextOperation: "DECLARE_STABLE_ID", BlockedBy: []string{"DECLARATION_ID"},
				})
				continue
			}
			if seen[declaration.StableID] {
				refutations = append(refutations, Refutation{
					Stage: "NORMALIZE", Step: "CHECK_STABLE_ID_UNIQUENESS", Reason: "DUPLICATE_STABLE_ID",
					Counterexample: declaration.StableID,
				})
			}
			seen[declaration.StableID] = true
		}
		required := phase.Normalization.Options["required_entity"]
		if required != "" && !seen[required] {
			unknowns = append(unknowns, Unknown{
				Stage: "NORMALIZE", Step: "REQUIRE_DECLARED_ENTITY", Reason: "REQUIRED_ENTITY_MISSING",
				UnknownClass: "DIRECT_MISSING", NextOperation: "ADD_REQUIRED_ENTITY", BlockedBy: []string{required},
			})
		}
	}
	if namespace == "" || namespace == "unknown" {
		namespace = "reflexive_unknown"
	}
	sort.SliceStable(declarations, func(left, right int) bool {
		if declarations[left].StableID != declarations[right].StableID {
			return declarations[left].StableID < declarations[right].StableID
		}
		if declarations[left].Kind != declarations[right].Kind {
			return declarations[left].Kind < declarations[right].Kind
		}
		return declarations[left].Name < declarations[right].Name
	})
	decision := reduce(unknowns, refutations)
	terminal := buildTerminalRecord(phase, decision, unknowns, refutations)
	ir := SemanticIR{
		Schema: irSchema, PhaseID: phase.ID, PhaseDigest: phase.Digest,
		OriginSourceDigest: sourceDigest, Namespace: namespace, Declarations: declarations,
		Evidence: TerminalEvidence{Unknowns: unknowns, Refutations: refutations}, TerminalRecord: terminal,
	}
	irBytes, err := marshalJSON(ir)
	if err != nil {
		return Receipt{}, err
	}
	generatedBytes := emitGenerated(ir, phase, terminal)
	terminalBytes, err := marshalJSON(terminal)
	if err != nil {
		return Receipt{}, err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return Receipt{}, err
	}
	irPath := filepath.Join(outputDir, "semantic-ir.json")
	generatedPath := filepath.Join(outputDir, "generated.go")
	terminalPath := filepath.Join(outputDir, "terminal-record.json")
	receiptPath := filepath.Join(outputDir, "receipt.json")
	if err := os.WriteFile(irPath, irBytes, 0o644); err != nil {
		return Receipt{}, err
	}
	if err := os.WriteFile(generatedPath, generatedBytes, 0o644); err != nil {
		return Receipt{}, err
	}
	if err := os.WriteFile(terminalPath, terminalBytes, 0o644); err != nil {
		return Receipt{}, err
	}
	receipt := Receipt{
		Schema: "gooo/reflexive-compiler-receipt/v2", RunID: options.RunID, Role: options.Role,
		InputKind: options.InputKind, Phase: FileLineage{Path: phasePath, Digest: phase.Digest},
		Source: FileLineage{Path: sourcePath, Digest: sourceDigest}, Input: FileLineage{Path: inputPath, Digest: inputDigest},
		SemanticIR: ArtifactLineage{Path: irPath, Digest: digestBytes(irBytes), Kind: "SEMANTIC_IR"},
		Generated:  ArtifactLineage{Path: generatedPath, Digest: digestBytes(generatedBytes), Kind: "BACKEND_ARTIFACT"},
		Terminal:   ArtifactLineage{Path: terminalPath, Digest: digestBytes(terminalBytes), Kind: "TERMINAL_RECORD"},
		Decision:   decision, Unknowns: unknowns, Refutations: refutations,
		TerminalRecord: terminal,
	}
	receipt.ExecutionDigest, err = digestJSON(executionProjection{
		Decision: decision, Unknowns: unknowns, Refutations: refutations,
		SemanticIR: receipt.SemanticIR.Digest, Generated: receipt.Generated.Digest,
		Terminal: receipt.Terminal.Digest,
	})
	if err != nil {
		return Receipt{}, err
	}
	receipt.ReceiptDigest, err = digestJSON(receipt)
	if err != nil {
		return Receipt{}, err
	}
	receiptBytes, err := marshalJSON(receipt)
	if err != nil {
		return Receipt{}, err
	}
	if err := os.WriteFile(receiptPath, receiptBytes, 0o644); err != nil {
		return Receipt{}, err
	}
	_ = phase.Backend
	_ = phase.Replay
	_ = sourceData
	return receipt, nil
}

func emitGenerated(ir SemanticIR, phase Phase, terminal TerminalRecord) []byte {
	var out strings.Builder
	authority := "meta/reflexive-normalize.gooo"
	if base := filepath.Base(phase.SourcePath); strings.HasSuffix(base, ".gooo") {
		authority = "meta/" + base
	}
	out.WriteString("// Code generated by gooo-reflexive-compiler-slice; DO NOT EDIT.\n")
	out.WriteString("// semantic-authority: ")
	out.WriteString(authority)
	out.WriteString("\n")
	out.WriteString("// backend-artifact: generated from semantic-ir.json\n")
	out.WriteString("package generated\n\n")
	out.WriteString("type SemanticNode struct {\n")
	out.WriteString("\tKind string\n\tStableID string\n\tName string\n\tParameters []string\n\tResult string\n")
	out.WriteString("}\n\nvar PhaseID = ")
	out.WriteString(strconv.Quote(phase.ID))
	out.WriteString("\nvar PhaseDigest = ")
	out.WriteString(strconv.Quote(phase.Digest))
	out.WriteString("\nvar OriginSourceDigest = ")
	out.WriteString(strconv.Quote(ir.OriginSourceDigest))
	out.WriteString("\n\nvar Nodes = []SemanticNode{\n")
	for _, declaration := range ir.Declarations {
		out.WriteString("\t{Kind: ")
		out.WriteString(strconv.Quote(declaration.Kind))
		out.WriteString(", StableID: ")
		out.WriteString(strconv.Quote(declaration.StableID))
		out.WriteString(", Name: ")
		out.WriteString(strconv.Quote(declaration.Name))
		if len(declaration.Parameters) > 0 {
			out.WriteString(", Parameters: []string{")
			for index, parameter := range declaration.Parameters {
				if index > 0 {
					out.WriteString(", ")
				}
				out.WriteString(strconv.Quote(parameter))
			}
			out.WriteString("}")
		}
		if declaration.Result != "" {
			out.WriteString(", Result: ")
			out.WriteString(strconv.Quote(declaration.Result))
		}
		out.WriteString("},\n")
	}
	out.WriteString("}\n")
	out.WriteString("\n// Terminal is the explanation-carrying result emitted with this backend.\n")
	out.WriteString("type TerminalRecord struct {\n")
	out.WriteString("\tDecision string\n\tStage string\n\tStep string\n\tReason string\n\tUnknownClass string\n\tNextOperation string\n\tBlockedBy []string\n\tCauseEdge FrontierEdge\n\tMinimalFrontier []FrontierEdge\n\tCounterexample string\n\tCounterexampleDigest string\n")
	out.WriteString("}\n\ntype FrontierEdge struct {\n\tFrom string\n\tTo string\n\tValueType string\n}\n\nvar Terminal = TerminalRecord{\n")
	out.WriteString("\tDecision: ")
	out.WriteString(strconv.Quote(terminal.Decision))
	out.WriteString(",\n\tStage: ")
	out.WriteString(strconv.Quote(terminal.Stage))
	out.WriteString(",\n\tStep: ")
	out.WriteString(strconv.Quote(terminal.Step))
	out.WriteString(",\n\tReason: ")
	out.WriteString(strconv.Quote(terminal.Reason))
	out.WriteString(",\n\tUnknownClass: ")
	out.WriteString(strconv.Quote(terminal.UnknownClass))
	out.WriteString(",\n\tNextOperation: ")
	out.WriteString(strconv.Quote(terminal.NextOperation))
	out.WriteString(",\n\tBlockedBy: ")
	out.WriteString(quoteStrings(terminal.BlockedBy))
	out.WriteString(",\n\tCauseEdge: FrontierEdge{From: ")
	out.WriteString(strconv.Quote(terminal.CauseEdge.From))
	out.WriteString(", To: ")
	out.WriteString(strconv.Quote(terminal.CauseEdge.To))
	out.WriteString(", ValueType: ")
	out.WriteString(strconv.Quote(terminal.CauseEdge.ValueType))
	out.WriteString("},\n\tMinimalFrontier: []FrontierEdge{")
	for index, edge := range terminal.MinimalFrontier {
		if index > 0 {
			out.WriteString(", ")
		}
		out.WriteString("{From: ")
		out.WriteString(strconv.Quote(edge.From))
		out.WriteString(", To: ")
		out.WriteString(strconv.Quote(edge.To))
		out.WriteString(", ValueType: ")
		out.WriteString(strconv.Quote(edge.ValueType))
		out.WriteString("}")
	}
	out.WriteString("},\n\tCounterexample: ")
	out.WriteString(strconv.Quote(terminal.Counterexample))
	out.WriteString(",\n\tCounterexampleDigest: ")
	out.WriteString(strconv.Quote(terminal.CounterexampleDigest))
	out.WriteString("\n}\n")
	return []byte(out.String())
}

func quoteStrings(values []string) string {
	var out strings.Builder
	out.WriteString("[]string{")
	for index, value := range values {
		if index > 0 {
			out.WriteString(", ")
		}
		out.WriteString(strconv.Quote(value))
	}
	out.WriteString("}")
	return out.String()
}

func marshalJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func digestJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return digestBytes(data), nil
}
