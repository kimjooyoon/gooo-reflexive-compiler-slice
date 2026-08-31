package compiler

const (
	DecisionClosed  = "CLOSED"
	DecisionUnknown = "UNKNOWN"
	DecisionRefuted = "REFUTED"
)

type Phase struct {
	ID            string
	Digest        string
	Normalization Operation
	Backend       Operation
	Replay        Operation
}

type Operation struct {
	Name    string
	Program string
	Options map[string]string
}

type Declaration struct {
	Kind       string   `json:"kind"`
	StableID   string   `json:"stable_id"`
	Name       string   `json:"name"`
	Parameters []string `json:"parameters,omitempty"`
	Result     string   `json:"result,omitempty"`
}

type SemanticIR struct {
	Schema             string        `json:"schema"`
	PhaseID            string        `json:"phase_id"`
	PhaseDigest        string        `json:"phase_digest"`
	OriginSourceDigest string        `json:"origin_source_digest"`
	Namespace          string        `json:"namespace"`
	Declarations       []Declaration `json:"declarations"`
}

type Unknown struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type Refutation struct {
	Stage          string `json:"stage"`
	Step           string `json:"step"`
	Reason         string `json:"reason"`
	Counterexample string `json:"counterexample"`
}

type FileLineage struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type ArtifactLineage struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Kind   string `json:"kind"`
}

type Receipt struct {
	Schema          string          `json:"schema"`
	RunID           string          `json:"run_id"`
	Role            string          `json:"role"`
	InputKind       string          `json:"input_kind"`
	Phase           FileLineage     `json:"phase"`
	Source          FileLineage     `json:"source"`
	Input           FileLineage     `json:"input"`
	SemanticIR      ArtifactLineage `json:"semantic_ir"`
	Generated       ArtifactLineage `json:"generated"`
	Decision        string          `json:"decision"`
	Unknowns        []Unknown       `json:"unknowns"`
	Refutations     []Refutation    `json:"refutations"`
	ExecutionDigest string          `json:"execution_digest"`
	ReceiptDigest   string          `json:"receipt_digest"`
}

func reduce(unknowns []Unknown, refutations []Refutation) string {
	if len(refutations) > 0 {
		return DecisionRefuted
	}
	if len(unknowns) > 0 {
		return DecisionUnknown
	}
	return DecisionClosed
}
