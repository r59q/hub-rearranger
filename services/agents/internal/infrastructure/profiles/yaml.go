package profiles

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

const MaxBytes = 65536
const maxDepth = 24

func parseCatalog(content []byte) (any, *domain.Diagnostic) {
	if len(content) > MaxBytes {
		return nil, problem("LIMIT_EXCEEDED", "Reduce the catalog to at most 65536 bytes.")
	}
	if !utf8.Valid(content) {
		return nil, invalidYAML()
	}

	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, invalidYAML()
	}
	var extra yaml.Node
	if decoder.Decode(&extra) != io.EOF {
		return nil, invalidYAML()
	}

	value, issue := nodeValue(document.Content[0], 1)
	if issue != nil {
		return nil, issue
	}

	// Normalize numeric values to JSON types; nonfinite YAML scalars fail safely.
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, invalidYAML()
	}
	var normalized any
	if json.Unmarshal(encoded, &normalized) != nil {
		return nil, invalidYAML()
	}
	return normalized, nil
}

func nodeValue(node *yaml.Node, depth int) (any, *domain.Diagnostic) {
	if depth > maxDepth {
		return nil, problem("LIMIT_EXCEEDED", "Reduce catalog nesting to 24 levels.")
	}
	if node.Anchor != "" || node.Kind == yaml.AliasNode {
		return nil, problem("INVALID_YAML", "Remove YAML anchors and aliases; use explicit values.")
	}
	if node.Style&yaml.TaggedStyle != 0 {
		return nil, problem("INVALID_YAML", "Remove explicit YAML tags.")
	}

	switch node.Kind {
	case yaml.MappingNode:
		result := map[string]any{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Tag != "!!str" || key.Value == "<<" {
				return nil, problem("INVALID_YAML", "Use string keys for catalog fields.")
			}
			if _, issue := nodeValue(key, depth+1); issue != nil {
				return nil, issue
			}
			if _, exists := result[key.Value]; exists {
				return nil, problem("DUPLICATE_KEY", "Remove duplicate mapping keys.")
			}

			value, issue := nodeValue(node.Content[i+1], depth+1)
			if issue != nil {
				return nil, issue
			}
			result[key.Value] = value
		}
		return result, nil
	case yaml.SequenceNode:
		result := make([]any, 0, len(node.Content))
		for _, item := range node.Content {
			value, issue := nodeValue(item, depth+1)
			if issue != nil {
				return nil, issue
			}
			result = append(result, value)
		}
		return result, nil
	case yaml.ScalarNode:
		if node.Tag == "!!timestamp" || (node.Tag == "!!bool" && node.Value != "true" && node.Value != "false") {
			return node.Value, nil
		}
		var value any
		if node.Decode(&value) != nil {
			return nil, invalidYAML()
		}
		return value, nil
	default:
		return nil, invalidYAML()
	}
}

func problem(code, message string) *domain.Diagnostic {
	return &domain.Diagnostic{Code: code, Path: "/", Message: message}
}

func invalidYAML() *domain.Diagnostic {
	return problem("INVALID_YAML", "Use one UTF-8 YAML document with supported scalar values.")
}

// ParseDocument applies the same bounded, unambiguous YAML policy to workflow metadata.
func ParseDocument(content []byte) (any, bool) {
	value, issue := parseCatalog(content)
	return value, issue == nil
}
