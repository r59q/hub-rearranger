// Package evidence validates the allowlisted, versioned diagnostic artifact.
package evidence

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"io"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/profiles"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema.v1.gen.json
var schemaJSON []byte

const MaxBytes = 16384

var ErrInvalid = errors.New("invalid diagnostic evidence")

type Decoder struct{ schema *jsonschema.Schema }

func NewDecoder() (*Decoder, error) {
	var document any
	if err := json.Unmarshal(schemaJSON, &document); err != nil {
		return nil, err
	}

	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource("evidence.json", document); err != nil {
		return nil, err
	}
	schema, err := compiler.Compile("evidence.json")
	if err != nil {
		return nil, err
	}

	return &Decoder{schema: schema}, nil
}

func (d *Decoder) Decode(content []byte) (*domain.RuntimeEvidence, error) {
	if len(content) > MaxBytes {
		return nil, ErrInvalid
	}

	// The shared YAML parser rejects duplicate JSON keys as well. JSON itself is
	// required here; schema validation constrains every publishable string.
	var jsonValue any
	value, valid := profiles.ParseDocument(content)
	if json.Unmarshal(content, &jsonValue) != nil || !valid || d.schema.Validate(value) != nil {
		return nil, ErrInvalid
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var result domain.RuntimeEvidence
	if decoder.Decode(&result) != nil {
		return nil, ErrInvalid
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, ErrInvalid
	}
	return &result, nil
}
