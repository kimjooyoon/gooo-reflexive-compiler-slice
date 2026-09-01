package audit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kimjooyoon/gooo-reflexive-compiler-slice/internal/compiler"
)

var unknownFields = []string{"stage", "step", "reason", "unknown_class", "next_operation", "blocked_by"}

type Options struct {
	PhasePath    string
	SourcePath   string
	BaselineDir  string
	CandidateDir string
	Expected     string
	Output       string
}

type unknown struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type refutation struct {
	Stage          string `json:"stage"`
	Step           string `json:"step"`
	Reason         string `json:"reason"`
	Counterexample string `json:"counterexample"`
}

type receipt struct {
	Schema         string          `json:"schema"`
	RunID          string          `json:"run_id"`
	Role           string          `json:"role"`
	InputKind      string          `json:"input_kind"`
	Phase          fileLineage     `json:"phase"`
	Source         fileLineage     `json:"source"`
	Input          fileLineage     `json:"input"`
	SemanticIR     artifactLineage `json:"semantic_ir"`
	Generated      artifactLineage `json:"generated"`
	Terminal       artifactLineage `json:"terminal"`
	Decision       string          `json:"decision"`
	Unknowns       []unknown       `json:"unknowns"`
	Refutations    []refutation    `json:"refutations"`
	TerminalRecord terminalRecord  `json:"terminal_record"`
}

type fileLineage struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type artifactLineage struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Kind   string `json:"kind"`
}

type frontierEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	ValueType string `json:"value_type"`
}

type terminalRecord struct {
	Schema               string         `json:"schema"`
	Decision             string         `json:"decision"`
	Stage                string         `json:"stage"`
	Step                 string         `json:"step"`
	Reason               string         `json:"reason"`
	UnknownClass         string         `json:"unknown_class"`
	NextOperation        string         `json:"next_operation"`
	BlockedBy            []string       `json:"blocked_by"`
	CauseEdge            frontierEdge   `json:"cause_edge"`
	MinimalFrontier      []frontierEdge `json:"minimal_frontier"`
	Counterexample       string         `json:"counterexample"`
	CounterexampleDigest string         `json:"counterexample_digest"`
}

type semanticIR struct {
	Schema             string `json:"schema"`
	PhaseID            string `json:"phase_id"`
	PhaseDigest        string `json:"phase_digest"`
	OriginSourceDigest string `json:"origin_source_digest"`
	Namespace          string `json:"namespace"`
	Declarations       []struct {
		Kind       string   `json:"kind"`
		StableID   string   `json:"stable_id"`
		Name       string   `json:"name"`
		Parameters []string `json:"parameters,omitempty"`
		Result     string   `json:"result,omitempty"`
	} `json:"declarations"`
	Evidence struct {
		Unknowns    []unknown    `json:"unknowns"`
		Refutations []refutation `json:"refutations"`
	} `json:"evidence"`
	TerminalRecord terminalRecord `json:"terminal_record"`
}

type lineageEvidence struct {
	Phase              fileLineage     `json:"phase"`
	Source             fileLineage     `json:"source"`
	BaselineIR         artifactLineage `json:"baseline_ir"`
	CandidateIR        artifactLineage `json:"candidate_ir"`
	BaselineGenerated  artifactLineage `json:"baseline_generated"`
	CandidateGenerated artifactLineage `json:"candidate_generated"`
	BaselineTerminal   artifactLineage `json:"baseline_terminal"`
	CandidateTerminal  artifactLineage `json:"candidate_terminal"`
}

type behavior struct {
	InputKind string `json:"input_kind"`
	Decision  string `json:"decision"`
	Digest    string `json:"digest"`
}

type rollback struct {
	Possible          bool   `json:"possible"`
	BaselineRetained  bool   `json:"baseline_retained"`
	CandidateSeparate bool   `json:"candidate_separate"`
	TargetDigest      string `json:"target_digest"`
}

type Report struct {
	Schema               string          `json:"schema"`
	ExpectedDecision     string          `json:"expected_decision"`
	Decision             string          `json:"decision"`
	Precedence           []string        `json:"precedence"`
	UnknownFields        []string        `json:"unknown_fields"`
	FirstExecutionDigest string          `json:"first_execution_digest"`
	RerunExecutionDigest string          `json:"rerun_execution_digest"`
	BaselineBehavior     behavior        `json:"baseline_behavior"`
	CandidateBehavior    behavior        `json:"candidate_behavior"`
	Rollback             rollback        `json:"rollback"`
	Lineage              lineageEvidence `json:"lineage"`
	TerminalRecord       terminalRecord  `json:"terminal_record"`
	Unknowns             []unknown       `json:"unknowns"`
	Refutations          []refutation    `json:"refutations"`
	Errors               []string        `json:"errors"`
}

type executionProjection struct {
	Decision    string
	Unknowns    []unknown
	Refutations []refutation
	SemanticIR  string
	Generated   string
	Terminal    string
}

func Verify(options Options) (Report, error) {
	report := Report{
		Schema:           "gooo/reflexive-independent-verification/v1",
		ExpectedDecision: options.Expected,
		Precedence:       []string{"REFUTED", "UNKNOWN", "CLOSED"},
		UnknownFields:    append([]string(nil), unknownFields...),
		Unknowns:         []unknown{}, Refutations: []refutation{}, Errors: []string{},
	}
	phasePath, err := filepath.Abs(options.PhasePath)
	if err != nil {
		return report, err
	}
	sourcePath, err := filepath.Abs(options.SourcePath)
	if err != nil {
		return report, err
	}
	baselineDir, err := filepath.Abs(options.BaselineDir)
	if err != nil {
		return report, err
	}
	candidateDir, err := filepath.Abs(options.CandidateDir)
	if err != nil {
		return report, err
	}
	phaseData, phaseDigest, err := readDigest(phasePath)
	if err != nil {
		return report, err
	}
	phase, err := compiler.LoadPhase(phasePath)
	if err != nil {
		return report, err
	}
	sourceData, sourceDigest, err := readDigest(sourcePath)
	if err != nil {
		return report, err
	}
	baseline, baselineRaw, err := readReceipt(filepath.Join(baselineDir, "receipt.json"))
	if err != nil {
		return report, err
	}
	candidate, candidateRaw, err := readReceipt(filepath.Join(candidateDir, "receipt.json"))
	if err != nil {
		return report, err
	}
	report.Errors = append(report.Errors, validateUnknownObjects(baselineRaw, "baseline")...)
	report.Errors = append(report.Errors, validateUnknownObjects(candidateRaw, "candidate")...)
	report.Errors = append(report.Errors, validateReceipt(baseline, "baseline", "source", phasePath, sourcePath, phaseDigest, sourceDigest)...)
	report.Errors = append(report.Errors, validateReceipt(candidate, "candidate", "semantic-ir", phasePath, sourcePath, phaseDigest, sourceDigest)...)

	baselineIRData, baselineIRDigest, err := readDigest(baseline.SemanticIR.Path)
	if err != nil {
		return report, err
	}
	candidateIRData, candidateIRDigest, err := readDigest(candidate.SemanticIR.Path)
	if err != nil {
		return report, err
	}
	baselineGenerated, baselineGeneratedDigest, err := readDigest(baseline.Generated.Path)
	if err != nil {
		return report, err
	}
	candidateGenerated, candidateGeneratedDigest, err := readDigest(candidate.Generated.Path)
	if err != nil {
		return report, err
	}
	baselineTerminalData, baselineTerminalDigest, err := readDigest(baseline.Terminal.Path)
	if err != nil {
		return report, err
	}
	candidateTerminalData, candidateTerminalDigest, err := readDigest(candidate.Terminal.Path)
	if err != nil {
		return report, err
	}
	var baselineTerminal terminalRecord
	if err := json.Unmarshal(baselineTerminalData, &baselineTerminal); err != nil {
		report.Errors = append(report.Errors, "baseline terminal record is not JSON")
	}
	var candidateTerminal terminalRecord
	if err := json.Unmarshal(candidateTerminalData, &candidateTerminal); err != nil {
		report.Errors = append(report.Errors, "candidate terminal record is not JSON")
	}
	report.Errors = append(report.Errors, validateTerminal(baseline, baselineTerminal, baselineTerminalData, baselineTerminalDigest, "baseline")...)
	report.Errors = append(report.Errors, validateTerminal(candidate, candidateTerminal, candidateTerminalData, candidateTerminalDigest, "candidate")...)
	report.Errors = append(report.Errors, validateFrontier(phase, baselineTerminal, "baseline")...)
	report.Errors = append(report.Errors, validateFrontier(phase, candidateTerminal, "candidate")...)
	if baseline.Terminal.Digest != baselineTerminalDigest || candidate.Terminal.Digest != candidateTerminalDigest {
		report.Errors = append(report.Errors, "terminal record lineage digest is not byte-derived")
	}
	if !bytes.Equal(baselineTerminalData, candidateTerminalData) {
		report.Errors = append(report.Errors, "first and rerun terminal record bytes differ")
	}
	var ir semanticIR
	if err := json.Unmarshal(baselineIRData, &ir); err != nil {
		report.Errors = append(report.Errors, "baseline semantic IR is not JSON")
	} else {
		report.Errors = append(report.Errors, validateIR(ir, phaseDigest, sourceDigest)...)
		report.Errors = append(report.Errors, validateIRTerminal(ir, baselineTerminal, "baseline")...)
		report.Errors = append(report.Errors, validateRefutationEvidence(ir, baseline.Refutations, "baseline")...)
		report.Errors = append(report.Errors, validateGenerated(baselineGenerated, ir, baselineTerminal, "baseline")...)
	}
	var candidateIR semanticIR
	if err := json.Unmarshal(candidateIRData, &candidateIR); err != nil {
		report.Errors = append(report.Errors, "candidate semantic IR is not JSON")
	} else {
		report.Errors = append(report.Errors, validateIR(candidateIR, phaseDigest, sourceDigest)...)
		report.Errors = append(report.Errors, validateIRTerminal(candidateIR, candidateTerminal, "candidate")...)
		report.Errors = append(report.Errors, validateRefutationEvidence(candidateIR, candidate.Refutations, "candidate")...)
		report.Errors = append(report.Errors, validateGenerated(candidateGenerated, candidateIR, candidateTerminal, "candidate")...)
	}
	if !bytes.Equal(baselineIRData, candidateIRData) {
		report.Errors = append(report.Errors, "first and rerun semantic IR bytes differ")
	}
	if !bytes.Equal(baselineGenerated, candidateGenerated) {
		report.Errors = append(report.Errors, "first and rerun generated backend bytes differ")
	}
	if candidate.Input.Path != baseline.SemanticIR.Path || candidate.Input.Digest != baselineIRDigest {
		report.Errors = append(report.Errors, "candidate input is not the first generated semantic IR")
	}
	if baseline.SemanticIR.Digest != baselineIRDigest || candidate.SemanticIR.Digest != candidateIRDigest {
		report.Errors = append(report.Errors, "semantic IR lineage digest is not byte-derived")
	}
	if baseline.Generated.Digest != baselineGeneratedDigest || candidate.Generated.Digest != candidateGeneratedDigest {
		report.Errors = append(report.Errors, "generated artifact lineage digest is not byte-derived")
	}

	baselineExecution := executionDigest(baseline, baselineIRDigest, baselineGeneratedDigest, baselineTerminalDigest)
	candidateExecution := executionDigest(candidate, candidateIRDigest, candidateGeneratedDigest, candidateTerminalDigest)
	report.FirstExecutionDigest = baselineExecution
	report.RerunExecutionDigest = candidateExecution
	report.BaselineBehavior = behavior{InputKind: baseline.InputKind, Decision: baseline.Decision, Digest: baselineExecution}
	report.CandidateBehavior = behavior{InputKind: candidate.InputKind, Decision: candidate.Decision, Digest: candidateExecution}
	if baselineExecution != candidateExecution {
		report.Errors = append(report.Errors, "baseline and candidate behavior digests differ")
	}
	report.Decision = reduce(baseline.Unknowns, baseline.Refutations)
	if candidate.Decision != reduce(candidate.Unknowns, candidate.Refutations) {
		report.Errors = append(report.Errors, "candidate precedence reduction differs")
	}
	if report.Decision != options.Expected {
		report.Errors = append(report.Errors, fmt.Sprintf("decision=%s, expected=%s", report.Decision, options.Expected))
	}
	report.Unknowns = append(report.Unknowns, baseline.Unknowns...)
	report.Refutations = append(report.Refutations, baseline.Refutations...)
	report.Rollback = rollback{
		Possible:          baselineIRDigest != "" && baselineGeneratedDigest != "" && baselineDir != candidateDir,
		BaselineRetained:  fileExists(filepath.Join(baselineDir, "semantic-ir.json")) && fileExists(filepath.Join(baselineDir, "generated.go")),
		CandidateSeparate: baselineDir != candidateDir,
		TargetDigest:      baselineIRDigest,
	}
	report.Lineage = lineageEvidence{
		Phase: fileLineage{Path: phasePath, Digest: phaseDigest}, Source: fileLineage{Path: sourcePath, Digest: sourceDigest},
		BaselineIR: baseline.SemanticIR, CandidateIR: candidate.SemanticIR,
		BaselineGenerated: baseline.Generated, CandidateGenerated: candidate.Generated,
		BaselineTerminal: baseline.Terminal, CandidateTerminal: candidate.Terminal,
	}
	report.TerminalRecord = baselineTerminal
	if report.Rollback.Possible && !report.Rollback.BaselineRetained {
		report.Errors = append(report.Errors, "rollback target was not retained")
	}
	if options.Output != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return report, err
		}
		if err := os.WriteFile(options.Output, append(data, '\n'), 0o644); err != nil {
			return report, err
		}
	}
	if len(report.Errors) > 0 {
		return report, fmt.Errorf("independent verification failed: %s", strings.Join(report.Errors, "; "))
	}
	_ = phaseData
	_ = sourceData
	return report, nil
}

func phaseGraphErrors(phase compiler.Phase) []string {
	errors := []string{}
	for _, issue := range phase.GraphUnknowns {
		errors = append(errors, "phase graph UNKNOWN: "+issue.Reason)
	}
	for _, issue := range phase.GraphRefutations {
		errors = append(errors, "phase graph REFUTED: "+issue.Reason)
	}
	return errors
}

func readReceipt(path string) (receipt, map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return receipt{}, nil, err
	}
	var value receipt
	if err := json.Unmarshal(data, &value); err != nil {
		return receipt{}, nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return receipt{}, nil, err
	}
	return value, raw, nil
}

func validateReceipt(value receipt, label, inputKind, phasePath, sourcePath, phaseDigest, sourceDigest string) []string {
	errors := []string{}
	if value.Schema != "gooo/reflexive-compiler-receipt/v2" {
		errors = append(errors, label+" receipt schema mismatch")
	}
	if value.Role != label {
		errors = append(errors, label+" role mismatch")
	}
	if value.InputKind != inputKind {
		errors = append(errors, label+" input kind mismatch")
	}
	if value.Phase.Path != phasePath || value.Phase.Digest != phaseDigest {
		errors = append(errors, label+" phase lineage mismatch")
	}
	if value.Source.Path != sourcePath || value.Source.Digest != sourceDigest {
		errors = append(errors, label+" source lineage mismatch")
	}
	if value.Decision != reduce(value.Unknowns, value.Refutations) {
		errors = append(errors, label+" decision does not follow precedence")
	}
	return errors
}

func validateUnknownObjects(raw map[string]json.RawMessage, label string) []string {
	errors := []string{}
	value, ok := raw["unknowns"]
	if !ok {
		return []string{label + " unknowns field is absent"}
	}
	var objects []map[string]json.RawMessage
	if err := json.Unmarshal(value, &objects); err != nil {
		return []string{label + " unknowns field is not an array"}
	}
	for index, object := range objects {
		for _, field := range unknownFields {
			if _, ok := object[field]; !ok {
				errors = append(errors, fmt.Sprintf("%s unknown[%d] missing %s", label, index, field))
			}
		}
	}
	return errors
}

func validateTerminal(value receipt, terminal terminalRecord, data []byte, digest, label string) []string {
	errors := []string{}
	if terminal.Schema != "gooo/reflexive-terminal-record/v1" {
		errors = append(errors, label+" terminal record schema mismatch")
	}
	if terminal.Decision != value.Decision || terminal.Decision != reduce(value.Unknowns, value.Refutations) {
		errors = append(errors, label+" terminal decision does not follow precedence")
	}
	if terminal.Stage == "" || terminal.Step == "" || terminal.Reason == "" || terminal.CounterexampleDigest == "" {
		errors = append(errors, label+" terminal explanation is incomplete")
	}
	if terminal.Decision == "UNKNOWN" && (terminal.UnknownClass == "" || terminal.NextOperation == "" || terminal.BlockedBy == nil) {
		errors = append(errors, label+" UNKNOWN terminal record is missing one of six required fields")
	}
	switch terminal.Decision {
	case "REFUTED":
		if len(value.Refutations) == 0 {
			errors = append(errors, label+" REFUTED terminal record has no refutation")
		} else {
			first := value.Refutations[0]
			if terminal.Stage != first.Stage || terminal.Step != first.Step || terminal.Reason != first.Reason || terminal.Counterexample != first.Counterexample {
				errors = append(errors, label+" terminal record does not preserve the first refutation")
			}
			if terminal.CounterexampleDigest != digestEvidence(first) {
				errors = append(errors, label+" terminal counterexample digest is not evidence-derived")
			}
		}
	case "UNKNOWN":
		if len(value.Unknowns) == 0 {
			errors = append(errors, label+" UNKNOWN terminal record has no unknown")
		} else {
			first := value.Unknowns[0]
			if terminal.Stage != first.Stage || terminal.Step != first.Step || terminal.Reason != first.Reason || terminal.UnknownClass != first.UnknownClass || terminal.NextOperation != first.NextOperation || !equalStringSlices(terminal.BlockedBy, first.BlockedBy) {
				errors = append(errors, label+" terminal record does not preserve the first UNKNOWN explanation")
			}
			if terminal.CounterexampleDigest != digestEvidence(first) {
				errors = append(errors, label+" terminal explanation digest is not evidence-derived")
			}
		}
	case "CLOSED":
		if terminal.Stage != "TERMINAL" || terminal.Step != "CLOSE_AFTER_VALIDATION" || terminal.Reason != "NO_UNKNOWN_OR_REFUTATION" || terminal.NextOperation != "RETAIN_RESULT" || len(terminal.BlockedBy) != 0 || terminal.Counterexample != "" {
			errors = append(errors, label+" CLOSED terminal explanation is not canonical")
		}
		closedEvidence := struct {
			Decision string `json:"decision"`
			Stage    string `json:"stage"`
			Step     string `json:"step"`
			Reason   string `json:"reason"`
		}{Decision: terminal.Decision, Stage: terminal.Stage, Step: terminal.Step, Reason: terminal.Reason}
		if terminal.CounterexampleDigest != digestEvidence(closedEvidence) {
			errors = append(errors, label+" CLOSED explanation digest is not evidence-derived")
		}
	}
	if digestBytes(data) != digest {
		errors = append(errors, label+" terminal record digest is not byte-derived")
	}
	return errors
}

func validateFrontier(phase compiler.Phase, terminal terminalRecord, label string) []string {
	if terminal.Decision == "CLOSED" {
		if len(terminal.MinimalFrontier) != 0 {
			return []string{label + " CLOSED terminal has a non-empty cause frontier"}
		}
		return nil
	}
	if len(terminal.MinimalFrontier) == 0 {
		return []string{label + " terminal record has no minimal frontier"}
	}
	if terminal.CauseEdge != terminal.MinimalFrontier[0] {
		return []string{label + " terminal cause edge is not the first minimal-frontier edge"}
	}
	errors := []string{}
	for index, edge := range terminal.MinimalFrontier {
		if edge.From == "" || edge.To == "" || edge.ValueType == "" {
			errors = append(errors, fmt.Sprintf("%s minimal frontier edge %d is incomplete", label, index))
		}
	}
	if terminal.Stage == "NORMALIZE" || terminal.Stage == "SOURCE_READ" || terminal.Stage == "SEMANTIC_IR_READ" {
		if len(phase.Activities) == 0 {
			errors = append(errors, label+" terminal frontier has no phase activity context")
		}
	}
	return errors
}

func digestEvidence(value any) string {
	data, _ := json.Marshal(value)
	return digestBytes(data)
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validateIR(value semanticIR, phaseDigest, sourceDigest string) []string {
	errors := []string{}
	if value.Schema != "gooo/reflexive-semantic-ir/v1" {
		errors = append(errors, "semantic IR schema mismatch")
	}
	if value.PhaseID != "reflexive.normalize.v1" || value.PhaseDigest != phaseDigest {
		errors = append(errors, "semantic IR phase lineage mismatch")
	}
	if value.OriginSourceDigest != sourceDigest {
		errors = append(errors, "semantic IR source lineage mismatch")
	}
	for _, declaration := range value.Declarations {
		if declaration.StableID == "" {
			errors = append(errors, "semantic IR contains empty stable id")
		}
	}
	return errors
}

func validateIRTerminal(value semanticIR, expected terminalRecord, label string) []string {
	actual, _ := json.Marshal(value.TerminalRecord)
	want, _ := json.Marshal(expected)
	if !bytes.Equal(actual, want) {
		return []string{label + " semantic IR terminal record differs from terminal artifact"}
	}
	if len(value.Evidence.Unknowns) == 0 && len(value.Evidence.Refutations) == 0 && expected.Decision != "CLOSED" {
		return []string{label + " semantic IR omitted terminal evidence"}
	}
	return nil
}

func validateRefutationEvidence(value semanticIR, refutations []refutation, label string) []string {
	seen := map[string]bool{}
	duplicates := map[string]bool{}
	for _, declaration := range value.Declarations {
		if seen[declaration.StableID] {
			duplicates[declaration.StableID] = true
		}
		seen[declaration.StableID] = true
	}
	hasDuplicateRefutation := false
	for _, value := range refutations {
		if value.Reason == "DUPLICATE_STABLE_ID" {
			hasDuplicateRefutation = true
			if !duplicates[value.Counterexample] {
				return []string{label + " refutation points to a non-duplicate stable id"}
			}
		}
	}
	if len(duplicates) > 0 && !hasDuplicateRefutation {
		return []string{label + " duplicate stable id was not preserved as refutation evidence"}
	}
	if len(duplicates) == 0 && hasDuplicateRefutation {
		return []string{label + " duplicate stable id refutation has no counterexample"}
	}
	return nil
}

func validateGenerated(data []byte, value semanticIR, terminal terminalRecord, label string) []string {
	errors := []string{}
	text := string(data)
	if !strings.Contains(text, "backend-artifact: generated from semantic-ir.json") || !strings.Contains(text, "semantic-authority: meta/reflexive-normalize") {
		errors = append(errors, label+" output is not marked as a derived backend artifact")
	}
	for _, field := range []string{"Decision", "Stage", "Step", "Reason", "UnknownClass", "NextOperation", "CounterexampleDigest"} {
		if !strings.Contains(text, field+": ") {
			errors = append(errors, label+" generated output omitted terminal field "+field)
		}
	}
	if !strings.Contains(text, "Decision: "+fmt.Sprintf("%q", terminal.Decision)) ||
		!strings.Contains(text, "Stage: "+fmt.Sprintf("%q", terminal.Stage)) ||
		!strings.Contains(text, "Step: "+fmt.Sprintf("%q", terminal.Step)) ||
		!strings.Contains(text, "CounterexampleDigest: "+fmt.Sprintf("%q", terminal.CounterexampleDigest)) {
		errors = append(errors, label+" generated output does not preserve terminal explanation")
	}
	for _, declaration := range value.Declarations {
		if !strings.Contains(text, fmt.Sprintf("StableID: %q", declaration.StableID)) {
			errors = append(errors, label+" generated output lost "+declaration.StableID)
		}
	}
	return errors
}

func readDigest(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(data)
	return data, "sha256:" + hex.EncodeToString(sum[:]), nil
}

func executionDigest(value receipt, irDigest, generatedDigest, terminalDigest string) string {
	data, _ := json.Marshal(executionProjection{
		Decision: value.Decision, Unknowns: value.Unknowns, Refutations: value.Refutations,
		SemanticIR: irDigest, Generated: generatedDigest, Terminal: terminalDigest,
	})
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func reduce(unknowns []unknown, refutations []refutation) string {
	if len(refutations) > 0 {
		return "REFUTED"
	}
	if len(unknowns) > 0 {
		return "UNKNOWN"
	}
	return "CLOSED"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
