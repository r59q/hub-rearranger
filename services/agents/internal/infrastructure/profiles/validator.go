// Package profiles validates using the canonical AW-003 schema, exported by
// make generate-agents-contract. It never interprets commands or persists data.
package profiles

import (
	_ "embed"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

//go:embed schema.v1.gen.json
var schemaJSON []byte

type Validator struct {
	schema   *jsonschema.Schema
	document map[string]any
	fields   map[string]bool
}

func NewValidator() (*Validator, error) {
	var document map[string]any
	if err := json.Unmarshal(schemaJSON, &document); err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseRegexpEngine(compileRegexp)
	if err := compiler.AddResource("schema.json", document); err != nil {
		return nil, err
	}
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, err
	}
	fields := map[string]bool{}
	collectFields(document, fields)
	return &Validator{schema: schema, document: document, fields: fields}, nil
}

func (v *Validator) Validate(content []byte) (map[string]map[string]any, []domain.Diagnostic) {
	value, issue := parseCatalog(content)
	if issue != nil {
		return nil, []domain.Diagnostic{*issue}
	}
	if catalog, ok := value.(map[string]any); ok {
		if version, ok := catalog["schema_version"].(float64); ok && version == float64(int(version)) && version != 1 {
			return nil, []domain.Diagnostic{{Code: "UNSUPPORTED_VERSION", Path: "/schema_version", Message: "Use schema_version: 1."}}
		}
	}
	if err := v.schema.Validate(value); err != nil {
		var validation *jsonschema.ValidationError
		if errors.As(err, &validation) {
			return nil, v.diagnostics(validation)
		}
		return nil, []domain.Diagnostic{{Code: "INVALID_TYPE", Path: "/", Message: "Use the field types defined in the v1 schema."}}
	}
	profiles := map[string]map[string]any{}
	for id, profile := range value.(map[string]any)["profiles"].(map[string]any) {
		profiles[id] = profile.(map[string]any)
	}
	return profiles, []domain.Diagnostic{}
}

// The canonical runner-label pattern needs ECMAScript negative lookahead.
// Only trusted schema patterns are compiled; repository values cannot add one.
type schemaRegexp struct{ *regexp2.Regexp }

func (r schemaRegexp) MatchString(value string) bool {
	matched, err := r.Regexp.MatchString(value)
	return err == nil && matched
}
func compileRegexp(pattern string) (jsonschema.Regexp, error) {
	compiled, err := regexp2.Compile(pattern, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}
	compiled.MatchTimeout = 50 * time.Millisecond
	return schemaRegexp{compiled}, nil
}

func collectFields(value any, fields map[string]bool) {
	switch node := value.(type) {
	case map[string]any:
		if properties, ok := node["properties"].(map[string]any); ok {
			for key := range properties {
				fields[key] = true
			}
		}
		for _, child := range node {
			collectFields(child, fields)
		}
	case []any:
		for _, child := range node {
			collectFields(child, fields)
		}
	}
}

func (v *Validator) safePath(parts []string) string {
	result := make([]string, len(parts))
	for index, part := range parts {
		switch {
		case index == 1 && parts[0] == "profiles":
			result[index] = "<profile>"
		case v.fields[part]:
			result[index] = part
		case index > 0 && (parts[index-1] == "sources" || parts[index-1] == "checks") && strings.Trim(part, "0123456789") == "" && part != "":
			result[index] = part
		default:
			result[index] = "<field>"
		}
	}
	return "/" + strings.Join(result, "/")
}
