package compiler

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	phaseSchema = "gooo/reflexive-compiler-phase/v1"
	irSchema    = "gooo/reflexive-semantic-ir/v1"
)

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func readBytes(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	return data, digestBytes(data), nil
}

type PhaseSummary struct {
	ID                 string       `json:"id"`
	Digest             string       `json:"digest"`
	Topology           string       `json:"topology"`
	ActivityRoles      []string     `json:"activity_roles"`
	ActivityCount      int          `json:"activity_count"`
	TypedEdgeCount     int          `json:"typed_edge_count"`
	Valid              bool         `json:"valid"`
	LocalizationStages int          `json:"localization_stages"`
	GraphUnknowns      []Unknown    `json:"graph_unknowns"`
	GraphRefutations   []Refutation `json:"graph_refutations"`
}

func LoadPhase(path string) (Phase, error) {
	return parsePhase(path)
}

func SummarizePhase(path string) (PhaseSummary, error) {
	phase, err := parsePhase(path)
	if err != nil {
		return PhaseSummary{}, err
	}
	roles := make([]string, 0, len(phase.Activities))
	for _, activity := range phase.Activities {
		roles = append(roles, activity.Name)
	}
	localizationStages := 0
	if containsRole(roles, "NormalizeSource") {
		localizationStages = 1
	}
	if containsRole(roles, "ParseSource") && containsRole(roles, "ValidateStableIDs") {
		localizationStages = 2
	}
	return PhaseSummary{
		ID: phase.ID, Digest: phase.Digest, Topology: phase.Topology,
		ActivityRoles: roles, ActivityCount: len(phase.Activities),
		TypedEdgeCount: len(phase.Edges), Valid: len(phase.GraphUnknowns) == 0 && len(phase.GraphRefutations) == 0,
		LocalizationStages: localizationStages, GraphUnknowns: phase.GraphUnknowns,
		GraphRefutations: phase.GraphRefutations,
	}, nil
}

func parsePhase(path string) (Phase, error) {
	data, digest, err := readBytes(path)
	if err != nil {
		return Phase{}, err
	}
	phase := Phase{
		SourcePath: path, ID: digestPhaseID(), Digest: digest,
		Precedence: []string{DecisionRefuted, DecisionUnknown, DecisionClosed},
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
			if line == "" {
				continue
			}
		}
		switch {
		case strings.HasPrefix(line, "topology "):
			phase.Topology = strings.TrimSpace(strings.TrimPrefix(line, "topology "))
		case strings.HasPrefix(line, "precedence "):
			phase.Precedence = parsePrecedence(strings.TrimSpace(strings.TrimPrefix(line, "precedence ")))
		case strings.HasPrefix(line, "authority "):
			phase.Authority = strings.TrimSpace(strings.TrimPrefix(line, "authority "))
		case strings.HasPrefix(line, "acceptance "):
			phase.Acceptance = strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "acceptance ")))
		case strings.HasPrefix(line, "rollback "):
			phase.Rollback = strings.TrimSpace(strings.TrimPrefix(line, "rollback "))
		case strings.HasPrefix(line, "split "):
			migration, ok := parseSplit(strings.TrimSpace(strings.TrimPrefix(line, "split ")))
			if !ok {
				phase.GraphUnknowns = append(phase.GraphUnknowns, phaseUnknown("PARSE", "READ_SPLIT_MIGRATION", "SPLIT_MIGRATION_NOT_PARSEABLE", "PARSE_PHASE_GRAPH", "REPAIR_SPLIT_MIGRATION", "SPLIT_MIGRATION"))
			} else {
				phase.Split = migration
			}
		case strings.HasPrefix(line, "edge "):
			edge, ok := parseEdge(strings.TrimSpace(strings.TrimPrefix(line, "edge ")))
			if !ok {
				phase.GraphUnknowns = append(phase.GraphUnknowns, phaseUnknown("PARSE", "READ_TYPED_EDGE", "TYPED_EDGE_NOT_PARSEABLE", "PARSE_PHASE_GRAPH", "REPAIR_TYPED_EDGE", "TYPED_EDGE"))
			} else {
				phase.Edges = append(phase.Edges, edge)
			}
		case strings.HasPrefix(line, "activity "):
			activity, ok := parsePhaseActivity(line)
			if !ok {
				phase.GraphUnknowns = append(phase.GraphUnknowns, phaseUnknown("PARSE", "READ_ACTIVITY", "ACTIVITY_NOT_PARSEABLE", "PARSE_PHASE_GRAPH", "REPAIR_ACTIVITY", "ACTIVITY"))
				continue
			}
			phase.Activities = append(phase.Activities, activity)
		}
	}
	if err := scanner.Err(); err != nil {
		return Phase{}, err
	}
	for _, activity := range phase.Activities {
		switch activity.Name {
		case "NormalizeSource", "ParseSource":
			if phase.Normalization.Program == "" {
				phase.Normalization = activity.Operation
			}
		case "ValidateStableIDs":
			if phase.Validation.Program == "" {
				phase.Validation = activity.Operation
			}
		case "EmitBackend":
			if phase.Backend.Program == "" {
				phase.Backend = activity.Operation
			}
		case "VerifyReplay":
			if phase.Replay.Program == "" {
				phase.Replay = activity.Operation
			}
		}
	}
	if phase.Topology == "" {
		if phase.Validation.Program != "" || hasActivity(phase.Activities, "ParseSource") {
			phase.Topology = "reflexive.normalize.v2"
		} else {
			phase.Topology = "reflexive.normalize.v1"
			phase.LegacyEdgesInferred = len(phase.Edges) == 0
		}
	}
	if phase.Topology == "reflexive.normalize.v1" && phase.LegacyEdgesInferred {
		phase.Edges = legacyEdges(phase.Activities)
	}
	if phase.Authority == "" && phase.Topology == "reflexive.normalize.v1" {
		phase.Authority = "SOURCE_GOOO_GRAPH"
	}
	if len(phase.Acceptance) == 0 && phase.Topology == "reflexive.normalize.v1" {
		phase.Acceptance = []string{DecisionClosed, DecisionUnknown, DecisionRefuted}
	}
	if phase.Rollback == "" && phase.Topology == "reflexive.normalize.v1" {
		phase.Rollback = "RETAIN_BASELINE"
	}
	phase.GraphUnknowns, phase.GraphRefutations = validatePhaseGraph(phase, phase.GraphUnknowns, phase.GraphRefutations)
	return phase, nil
}

func digestPhaseID() string {
	return "reflexive.normalize.v1"
}

func parsePhaseActivity(line string) (Activity, bool) {
	computed := strings.Index(line, " computes ")
	if computed < 0 {
		return Activity{}, false
	}
	left := strings.TrimSpace(strings.TrimPrefix(line[:computed], "activity "))
	nameEnd := strings.IndexByte(left, '(')
	if nameEnd < 1 {
		return Activity{}, false
	}
	right := strings.IndexByte(left[nameEnd+1:], ')')
	if right < 0 {
		return Activity{}, false
	}
	right += nameEnd + 1
	if !strings.HasPrefix(strings.TrimSpace(left[right+1:]), "->") {
		return Activity{}, false
	}
	inputType := strings.TrimSpace(left[nameEnd+1 : right])
	outputType := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(left[right+1:]), "->"))
	if strings.Contains(outputType, " ") || inputType == "" || outputType == "" {
		return Activity{}, false
	}
	program, err := strconv.Unquote(strings.TrimSpace(line[computed+len(" computes "):]))
	if err != nil {
		return Activity{}, false
	}
	name := strings.TrimSpace(left[:nameEnd])
	if name == "" || program == "" {
		return Activity{}, false
	}
	return Activity{
		Name: name, InputType: inputType, OutputType: outputType,
		Operation: Operation{Name: name, Program: program, Options: parseProgram(program)},
	}, true
}

func parseEdge(value string) (Edge, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return Edge{}, false
	}
	roles := strings.Fields(strings.TrimSpace(parts[0]))
	if len(roles) != 3 || roles[1] != "->" {
		return Edge{}, false
	}
	valueType := strings.TrimSpace(parts[1])
	if roles[0] == "" || roles[2] == "" || valueType == "" || strings.Contains(valueType, " ") {
		return Edge{}, false
	}
	return Edge{From: roles[0], To: roles[2], ValueType: valueType}, true
}

func parseSplit(value string) (SplitMigration, bool) {
	parts := strings.Split(value, "->")
	if len(parts) != 2 {
		return SplitMigration{}, false
	}
	retired := strings.TrimSpace(parts[0])
	added := strings.Split(strings.TrimSpace(parts[1]), "+")
	if retired == "" || len(added) != 2 || strings.TrimSpace(added[0]) == "" || strings.TrimSpace(added[1]) == "" {
		return SplitMigration{}, false
	}
	return SplitMigration{Retired: retired, Added: []string{strings.TrimSpace(added[0]), strings.TrimSpace(added[1])}}, true
}

func parsePrecedence(value string) []string {
	parts := strings.Fields(strings.ReplaceAll(value, ">", " "))
	return parts
}

func legacyEdges(activities []Activity) []Edge {
	byName := map[string]Activity{}
	for _, activity := range activities {
		byName[activity.Name] = activity
	}
	return []Edge{
		{From: "NormalizeSource", To: "EmitBackend", ValueType: byName["NormalizeSource"].OutputType},
		{From: "NormalizeSource", To: "VerifyReplay", ValueType: byName["VerifyReplay"].InputType},
	}
}

func validatePhaseGraph(phase Phase, unknowns []Unknown, refutations []Refutation) ([]Unknown, []Refutation) {
	if !equalStrings(phase.Precedence, []string{DecisionRefuted, DecisionUnknown, DecisionClosed}) {
		refutations = append(refutations, phaseRefutation("CHECK_PRECEDENCE", "INVALID_PRECEDENCE", strings.Join(phase.Precedence, ">")))
	}
	if !equalStrings(phase.Acceptance, []string{DecisionClosed, DecisionUnknown, DecisionRefuted}) {
		refutations = append(refutations, phaseRefutation("CHECK_ACCEPTANCE", "INVALID_ACCEPTANCE_STATES", strings.Join(phase.Acceptance, ",")))
	}
	if phase.Rollback != "RETAIN_BASELINE" {
		refutations = append(refutations, phaseRefutation("CHECK_ROLLBACK", "INVALID_ROLLBACK_POLICY", phase.Rollback))
	}
	counts := map[string]int{}
	stableIDs := map[string]bool{}
	for _, activity := range phase.Activities {
		counts[activity.Name]++
		stableID := "reflexive://activity/" + kebab(activity.Name)
		if stableIDs[stableID] {
			refutations = append(refutations, phaseRefutation("CHECK_STABLE_ID_UNIQUENESS", "DUPLICATE_STABLE_ID", stableID))
		}
		stableIDs[stableID] = true
	}

	roles := []string{"NormalizeSource", "EmitBackend", "VerifyReplay"}
	if phase.Topology == "reflexive.normalize.v2" {
		roles = []string{"ParseSource", "ValidateStableIDs", "EmitBackend", "VerifyReplay"}
	} else if phase.Topology != "reflexive.normalize.v1" {
		refutations = append(refutations, phaseRefutation("CHECK_TOPOLOGY", "INVALID_TOPOLOGY", phase.Topology))
	}
	for _, role := range roles {
		if counts[role] == 0 {
			unknowns = append(unknowns, phaseUnknown("PHASE_GRAPH", "REQUIRE_ROLE", "REQUIRED_ROLE_MISSING", "DIRECT_MISSING", "ADD_PHASE_ROLE", role))
		}
		if counts[role] > 1 {
			refutations = append(refutations, phaseRefutation("CHECK_ROLE_UNIQUENESS", "DUPLICATE_PHASE_ROLE", role))
		}
	}
	for role, count := range counts {
		if count > 0 && !containsString(roles, role) {
			refutations = append(refutations, phaseRefutation("CHECK_ROLE_SET", "UNSUPPORTED_PHASE_ROLE", role))
		}
	}

	if phase.Authority == "" {
		unknowns = append(unknowns, phaseUnknown("PHASE_GRAPH", "REQUIRE_AUTHORITY", "SOURCE_GOOO_GRAPH_AUTHORITY_MISSING", "DIRECT_MISSING", "DECLARE_GRAPH_AUTHORITY", "SOURCE_GOOO_GRAPH"))
	} else if phase.Authority != "SOURCE_GOOO_GRAPH" {
		refutations = append(refutations, phaseRefutation("CHECK_AUTHORITY", "INVALID_PHASE_AUTHORITY", phase.Authority))
	}
	if phase.Backend.Program != "" && phase.Backend.Options["authority"] != "SOURCE_GOOO_GRAPH" {
		refutations = append(refutations, phaseRefutation("CHECK_BACKEND_AUTHORITY", "INVALID_BACKEND_AUTHORITY", phase.Backend.Options["authority"]))
	}
	if phase.Backend.Program != "" && phase.Backend.Options["artifact"] != "BACKEND_ONLY" {
		refutations = append(refutations, phaseRefutation("CHECK_BACKEND_ARTIFACT", "INVALID_BACKEND_ARTIFACT", phase.Backend.Options["artifact"]))
	}
	if phase.Replay.Program != "" && (phase.Replay.Options["input"] != "GENERATED_SEMANTIC_IR" || phase.Replay.Options["rollback"] != "RETAIN_BASELINE") {
		refutations = append(refutations, phaseRefutation("CHECK_REPLAY_CONTRACT", "INVALID_REPLAY_CONTRACT", phase.Replay.Program))
	}
	if phase.Normalization.Program != "" && (phase.Normalization.Options["input"] != "GOOO" || phase.Normalization.Options["normal_form"] != "sort-by-stable-id") {
		refutations = append(refutations, phaseRefutation("CHECK_NORMALIZATION_CONTRACT", "INVALID_NORMALIZATION_CONTRACT", phase.Normalization.Program))
	}

	requiredEdges := []Edge{
		{From: "NormalizeSource", To: "EmitBackend", ValueType: "SemanticIR"},
		{From: "NormalizeSource", To: "VerifyReplay", ValueType: "SemanticIR"},
	}
	if phase.Topology == "reflexive.normalize.v2" {
		requiredEdges = []Edge{
			{From: "ParseSource", To: "ValidateStableIDs", ValueType: "ParsedSource"},
			{From: "ValidateStableIDs", To: "EmitBackend", ValueType: "SemanticIR"},
			{From: "ValidateStableIDs", To: "VerifyReplay", ValueType: "SemanticIR"},
		}
		if phase.Split.Retired != "NormalizeSource" || !equalStrings(phase.Split.Added, []string{"ParseSource", "ValidateStableIDs"}) {
			unknowns = append(unknowns, phaseUnknown("PHASE_GRAPH", "REQUIRE_SPLIT_MIGRATION", "SPLIT_MIGRATION_MISSING", "DIRECT_MISSING", "APPLY_SPLIT_MIGRATION", "NormalizeSource->ParseSource+ValidateStableIDs"))
		}
	}
	edgeSeen := map[string]bool{}
	adjacency := map[string][]string{}
	for _, edge := range phase.Edges {
		key := edge.From + "->" + edge.To + ":" + edge.ValueType
		if edgeSeen[key] {
			refutations = append(refutations, phaseRefutation("CHECK_EDGE_UNIQUENESS", "DUPLICATE_TYPED_EDGE", key))
		}
		edgeSeen[key] = true
		from := activityByName(phase.Activities, edge.From)
		to := activityByName(phase.Activities, edge.To)
		if from.Name == "" || to.Name == "" {
			refutations = append(refutations, phaseRefutation("CHECK_TYPED_EDGE", "EDGE_ROLE_NOT_FOUND", key))
			continue
		}
		if from.OutputType != edge.ValueType || to.InputType != edge.ValueType {
			refutations = append(refutations, phaseRefutation("CHECK_TYPED_EDGE", "INVALID_TYPED_EDGE", key))
		}
		adjacency[edge.From] = append(adjacency[edge.From], edge.To)
	}
	if phase.Topology == "reflexive.normalize.v2" && len(phase.Edges) == 0 {
		for _, edge := range requiredEdges {
			unknowns = append(unknowns, phaseUnknown("PHASE_GRAPH", "REQUIRE_TYPED_EDGE", "REQUIRED_TYPED_EDGE_MISSING", "DIRECT_MISSING", "ADD_TYPED_EDGE", edge.From+"->"+edge.To+":"+edge.ValueType))
		}
	}
	for _, required := range requiredEdges {
		key := required.From + "->" + required.To + ":" + required.ValueType
		if !edgeSeen[key] {
			unknowns = append(unknowns, phaseUnknown("PHASE_GRAPH", "REQUIRE_TYPED_EDGE", "REQUIRED_TYPED_EDGE_MISSING", "DIRECT_MISSING", "ADD_TYPED_EDGE", key))
		}
	}
	if cycle := findCycle(adjacency, roles); cycle != "" {
		refutations = append(refutations, phaseRefutation("CHECK_ACYCLIC_REACHABILITY", "CYCLE_IN_PHASE_GRAPH", cycle))
	}
	reachable := reachableRoles(adjacency, roles)
	for _, role := range roles {
		if !reachable[role] && counts[role] > 0 {
			unknowns = append(unknowns, phaseUnknown("PHASE_GRAPH", "CHECK_REACHABILITY", "ROLE_NOT_REACHABLE", "DIRECT_MISSING", "CONNECT_PHASE_ROLE", role))
		}
	}
	if evidence := activityByName(phase.Activities, "VerifyReplay"); evidence.Name != "" && evidence.OutputType != "Evidence" {
		refutations = append(refutations, phaseRefutation("CHECK_TERMINAL_EVIDENCE", "INVALID_TERMINAL_EVIDENCE_TYPE", evidence.OutputType))
	}
	return unknowns, refutations
}

func phaseUnknown(step, substep, reason, class, next, blocked string) Unknown {
	return Unknown{Stage: step, Step: substep, Reason: reason, UnknownClass: class, NextOperation: next, BlockedBy: []string{blocked}}
}

func phaseRefutation(step, reason, counterexample string) Refutation {
	return Refutation{Stage: "PHASE_GRAPH", Step: step, Reason: reason, Counterexample: counterexample}
}

func activityByName(activities []Activity, name string) Activity {
	for _, activity := range activities {
		if activity.Name == name {
			return activity
		}
	}
	return Activity{}
}

func hasActivity(activities []Activity, name string) bool {
	return activityByName(activities, name).Name != ""
}

func containsRole(roles []string, name string) bool {
	return containsString(roles, name)
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func equalStrings(left, right []string) bool {
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

func findCycle(adjacency map[string][]string, roles []string) string {
	state := map[string]int{}
	stack := []string{}
	var visit func(string) string
	visit = func(node string) string {
		state[node] = 1
		stack = append(stack, node)
		for _, next := range adjacency[node] {
			if state[next] == 1 {
				start := 0
				for index, value := range stack {
					if value == next {
						start = index
						break
					}
				}
				return strings.Join(append(stack[start:], next), "->")
			}
			if state[next] == 0 {
				if cycle := visit(next); cycle != "" {
					return cycle
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[node] = 2
		return ""
	}
	for _, role := range roles {
		if state[role] == 0 {
			if cycle := visit(role); cycle != "" {
				return cycle
			}
		}
	}
	return ""
}

func reachableRoles(adjacency map[string][]string, roles []string) map[string]bool {
	indegree := map[string]int{}
	for _, role := range roles {
		indegree[role] = 0
	}
	for from, targets := range adjacency {
		if !containsString(roles, from) {
			continue
		}
		for _, target := range targets {
			if containsString(roles, target) {
				indegree[target]++
			}
		}
	}
	queue := []string{}
	for _, role := range roles {
		if indegree[role] == 0 {
			queue = append(queue, role)
		}
	}
	reachable := map[string]bool{}
	for len(queue) > 0 {
		role := queue[0]
		queue = queue[1:]
		if reachable[role] {
			continue
		}
		reachable[role] = true
		for _, next := range adjacency[role] {
			queue = append(queue, next)
		}
	}
	return reachable
}

func parseProgram(value string) map[string]string {
	result := map[string]string{}
	parts := strings.Split(value, ";")
	for _, part := range parts[1:] {
		key, value, ok := strings.Cut(part, "=")
		if ok && key != "" {
			result[key] = value
		}
	}
	return result
}

func parseActivityProgram(line string) (string, string, bool) {
	computed := strings.Index(line, " computes ")
	if computed < 0 {
		return "", "", false
	}
	left := strings.TrimSpace(strings.TrimPrefix(line[:computed], "activity "))
	nameEnd := strings.IndexByte(left, '(')
	if nameEnd < 1 {
		return "", "", false
	}
	program, err := strconv.Unquote(strings.TrimSpace(line[computed+len(" computes "):]))
	if err != nil {
		return "", "", false
	}
	return left[:nameEnd], program, true
}

func parseSource(data []byte) (string, []Declaration, error) {
	var namespace string
	declarations := []Declaration{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "package ") {
			continue
		}
		if strings.HasPrefix(line, "namespace ") {
			fields := strings.Fields(line)
			if len(fields) != 2 || namespace != "" {
				return "", nil, fmt.Errorf("line %d: invalid namespace", lineNumber)
			}
			namespace = fields[1]
			continue
		}
		if strings.HasPrefix(line, "entity ") {
			declaration, err := parseEntity(line)
			if err != nil {
				return "", nil, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			declarations = append(declarations, declaration)
			continue
		}
		if strings.HasPrefix(line, "activity ") {
			declaration, err := parseActivity(line, namespace)
			if err != nil {
				return "", nil, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			declarations = append(declarations, declaration)
			continue
		}
		return "", nil, fmt.Errorf("line %d: unsupported declaration", lineNumber)
	}
	if err := scanner.Err(); err != nil {
		return "", nil, err
	}
	if namespace == "" {
		return "", nil, fmt.Errorf("namespace is required")
	}
	return namespace, declarations, nil
}

func sourceUnknownClass(err error) string {
	message := ""
	if err != nil {
		message = err.Error()
	}
	switch {
	case strings.Contains(message, "namespace is required"):
		return "SYNTAX_ERROR"
	case strings.Contains(message, "invalid namespace"):
		return "DUPLICATE_NAMESPACE"
	case strings.Contains(message, "unsupported declaration"):
		return "UNSUPPORTED_DECLARATION"
	case strings.Contains(message, "entity stable id is invalid"):
		return "INVALID_DECLARATION"
	case strings.Contains(message, "entity must") || strings.Contains(message, "activity signature"):
		return "INVALID_DECLARATION"
	default:
		return "SOURCE_PARSE_ERROR"
	}
}

func parseEntity(line string) (Declaration, error) {
	fields := strings.Fields(line)
	if len(fields) != 4 || fields[0] != "entity" || fields[2] != "id" {
		return Declaration{}, fmt.Errorf("entity must be 'entity Name id \"stable-id\"'")
	}
	id, err := strconv.Unquote(fields[3])
	if err != nil || id == "" {
		return Declaration{}, fmt.Errorf("entity stable id is invalid")
	}
	return Declaration{Kind: "entity", StableID: id, Name: fields[1]}, nil
}

func parseActivity(line, namespace string) (Declaration, error) {
	body := strings.TrimSpace(strings.TrimPrefix(line, "activity "))
	left := strings.IndexByte(body, '(')
	right := strings.IndexByte(body, ')')
	if left < 1 || right < left || !strings.HasPrefix(strings.TrimSpace(body[right+1:]), "->") {
		return Declaration{}, fmt.Errorf("activity signature is invalid")
	}
	name := body[:left]
	parameterText := body[left+1 : right]
	parameters := []string{}
	if strings.TrimSpace(parameterText) != "" {
		for _, value := range strings.Split(parameterText, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				return Declaration{}, fmt.Errorf("activity parameter is empty")
			}
			parameters = append(parameters, value)
		}
	}
	tail := strings.TrimSpace(body[right+1:])
	tail = strings.TrimSpace(strings.TrimPrefix(tail, "->"))
	if computes := strings.Index(tail, " computes "); computes >= 0 {
		tail = strings.TrimSpace(tail[:computes])
	}
	if tail == "" {
		return Declaration{}, fmt.Errorf("activity result is empty")
	}
	return Declaration{
		Kind: "activity", StableID: namespace + "://activity/" + kebab(name), Name: name,
		Parameters: parameters, Result: tail,
	}, nil
}

func kebab(value string) string {
	var out strings.Builder
	for index, r := range value {
		if r >= 'A' && r <= 'Z' {
			if index > 0 {
				out.WriteByte('-')
			}
			out.WriteRune(r + ('a' - 'A'))
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func loadIR(path string) (SemanticIR, []byte, string, error) {
	data, digest, err := readBytes(path)
	if err != nil {
		return SemanticIR{}, nil, "", err
	}
	var ir SemanticIR
	if err := json.Unmarshal(data, &ir); err != nil {
		return SemanticIR{}, nil, "", err
	}
	if ir.Schema != irSchema || ir.PhaseID == "" || ir.OriginSourceDigest == "" || ir.Namespace == "" {
		return SemanticIR{}, nil, "", fmt.Errorf("semantic IR envelope is incomplete")
	}
	return ir, data, digest, nil
}
