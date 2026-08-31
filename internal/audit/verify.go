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
	Schema      string          `json:"schema"`
	RunID       string          `json:"run_id"`
	Role        string          `json:"role"`
	InputKind   string          `json:"input_kind"`
	Phase       fileLineage     `json:"phase"`
	Source      fileLineage     `json:"source"`
	Input       fileLineage     `json:"input"`
	SemanticIR  artifactLineage `json:"semantic_ir"`
	Generated   artifactLineage `json:"generated"`
	Decision    string          `json:"decision"`
	Unknowns    []unknown       `json:"unknowns"`
	Refutations []refutation    `json:"refutations"`
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
}

type lineageEvidence struct {
	Phase              fileLineage     `json:"phase"`
	Source             fileLineage     `json:"source"`
	BaselineIR         artifactLineage `json:"baseline_ir"`
	CandidateIR        artifactLineage `json:"candidate_ir"`
	BaselineGenerated  artifactLineage `json:"baseline_generated"`
	CandidateGenerated artifactLineage `json:"candidate_generated"`
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
	var ir semanticIR
	if err := json.Unmarshal(baselineIRData, &ir); err != nil {
		report.Errors = append(report.Errors, "baseline semantic IR is not JSON")
	} else {
		report.Errors = append(report.Errors, validateIR(ir, phaseDigest, sourceDigest)...)
		report.Errors = append(report.Errors, validateRefutationEvidence(ir, baseline.Refutations, "baseline")...)
		report.Errors = append(report.Errors, validateGenerated(baselineGenerated, ir, "baseline")...)
	}
	var candidateIR semanticIR
	if err := json.Unmarshal(candidateIRData, &candidateIR); err != nil {
		report.Errors = append(report.Errors, "candidate semantic IR is not JSON")
	} else {
		report.Errors = append(report.Errors, validateIR(candidateIR, phaseDigest, sourceDigest)...)
		report.Errors = append(report.Errors, validateRefutationEvidence(candidateIR, candidate.Refutations, "candidate")...)
		report.Errors = append(report.Errors, validateGenerated(candidateGenerated, candidateIR, "candidate")...)
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

	baselineExecution := executionDigest(baseline, baselineIRDigest, baselineGeneratedDigest)
	candidateExecution := executionDigest(candidate, candidateIRDigest, candidateGeneratedDigest)
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
	}
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

func validateGenerated(data []byte, value semanticIR, label string) []string {
	errors := []string{}
	text := string(data)
	if !strings.Contains(text, "backend-artifact: generated from semantic-ir.json") || !strings.Contains(text, "semantic-authority: meta/reflexive-normalize.gooo") {
		errors = append(errors, label+" output is not marked as a derived backend artifact")
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

func executionDigest(value receipt, irDigest, generatedDigest string) string {
	data, _ := json.Marshal(executionProjection{
		Decision: value.Decision, Unknowns: value.Unknowns, Refutations: value.Refutations,
		SemanticIR: irDigest, Generated: generatedDigest,
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
