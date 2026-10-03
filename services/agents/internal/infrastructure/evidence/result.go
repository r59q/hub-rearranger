package evidence

import (
	_ "embed"
	"encoding/json"

	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/profiles"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed result.v1.gen.json
var resultSchema []byte

type PatchResult struct {
	AssignmentID    string `json:"assignment_id"`
	ProfileRevision string `json:"profile_revision"`
	BaseSHA         string `json:"base_sha"`
	RunID           int64  `json:"run_id"`
	Attempt         int    `json:"run_attempt"`
	Outcome         string `json:"outcome"`
	Proposal        *struct {
		PatchSHA256   string `json:"patch_sha256"`
		SummarySHA256 string `json:"summary_sha256"`
	} `json:"proposal"`
	Validation []struct {
		Outcome string `json:"outcome"`
	} `json:"validation"`
}

type ResultDecoder struct{ schema *jsonschema.Schema }

func NewResultDecoder() (*ResultDecoder, error) {
	var document any
	if err := json.Unmarshal(resultSchema, &document); err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("result.json", document); err != nil {
		return nil, err
	}
	schema, err := compiler.Compile("result.json")
	return &ResultDecoder{schema: schema}, err
}

func (d *ResultDecoder) Decode(content []byte) (PatchResult, error) {
	var result PatchResult
	var document any
	value, valid := profiles.ParseDocument(content)
	if len(content) > 65536 || !valid || json.Unmarshal(content, &document) != nil || d.schema.Validate(value) != nil || json.Unmarshal(content, &result) != nil {
		return result, ErrInvalid
	}
	return result, nil
}
