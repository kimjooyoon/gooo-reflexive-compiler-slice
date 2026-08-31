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

func parsePhase(path string) (Phase, error) {
	data, digest, err := readBytes(path)
	if err != nil {
		return Phase{}, err
	}
	var normalize, backend, replay Operation
	seen := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "activity ") {
			name, program, ok := parseActivityProgram(line)
			if !ok {
				continue
			}
			operation := Operation{Name: name, Program: program, Options: parseProgram(program)}
			switch {
			case strings.HasPrefix(program, "reflexive.normalize:v1"):
				normalize = operation
			case strings.HasPrefix(program, "reflexive.backend-go:v1"):
				backend = operation
			case strings.HasPrefix(program, "reflexive.replay:v1"):
				replay = operation
			}
			seen[name] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return Phase{}, err
	}
	if len(seen) != 3 || normalize.Program == "" || backend.Program == "" || replay.Program == "" {
		return Phase{}, fmt.Errorf("phase graph must declare exactly three executable activities")
	}
	if normalize.Options["authority"] != "" {
		return Phase{}, fmt.Errorf("normalization activity has unexpected authority option")
	}
	if normalize.Options["input"] != "GOOO" || normalize.Options["normal_form"] != "sort-by-stable-id" {
		return Phase{}, fmt.Errorf("phase graph does not declare the released Gooo normalization rule")
	}
	if backend.Options["authority"] != "SOURCE_GOOO_GRAPH" || backend.Options["artifact"] != "BACKEND_ONLY" {
		return Phase{}, fmt.Errorf("backend activity is not explicitly derived-only")
	}
	if replay.Options["input"] != "GENERATED_SEMANTIC_IR" || replay.Options["rollback"] != "RETAIN_BASELINE" {
		return Phase{}, fmt.Errorf("replay activity does not declare generated-IR input and rollback")
	}
	return Phase{ID: "reflexive.normalize.v1", Digest: digest, Normalization: normalize, Backend: backend, Replay: replay}, nil
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
