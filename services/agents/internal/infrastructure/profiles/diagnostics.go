package profiles

import (
	"fmt"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

func (v *Validator) diagnostics(root *jsonschema.ValidationError) []domain.Diagnostic {
	unique := map[domain.Diagnostic]bool{}
	var visit func(*jsonschema.ValidationError)
	visit = func(err *jsonschema.ValidationError) {
		add := func(code, message string, parts []string) {
			unique[domain.Diagnostic{Code: code, Path: v.safePath(parts), Message: message}] = true
		}
		parts := err.InstanceLocation
		switch issue := err.ErrorKind.(type) {
		case *kind.Required:
			for _, field := range issue.Missing {
				add("MISSING_FIELD", "Add this required field.", append(append([]string{}, parts...), field))
			}
		case *kind.Type:
			add("INVALID_TYPE", "Use type: "+strings.Join(issue.Want, ", ")+".", parts)
		case *kind.Const:
			add("UNSUPPORTED_VALUE", fmt.Sprintf("Use value: %v.", issue.Want), parts)
		case *kind.Enum:
			expected := make([]string, len(issue.Want))
			for i, value := range issue.Want {
				expected[i] = fmt.Sprint(value)
			}
			add("UNSUPPORTED_VALUE", "Choose from: "+strings.Join(expected, ", ")+".", parts)
		case *kind.AdditionalProperties:
			add("UNSUPPORTED_FIELD", "Remove fields not defined in the v1 schema.", parts)
		case *kind.Pattern:
			add("INVALID_FORMAT", "Use the format defined in the v1 schema.", parts)
		case *kind.UniqueItems:
			add("DUPLICATE_VALUE", "Remove duplicate list values.", parts)
		case *kind.Contains:
			// Contains causes are per-item mismatches, not invalid item values. Report
			// the required schema-owned source once, like the offline validator.
			add("MISSING_CONTEXT", "Include context source: "+v.containsValue(err.SchemaURL)+".", parts)
		default:
			if len(err.Causes) > 0 {
				for _, cause := range err.Causes {
					visit(cause)
				}
			} else {
				add("LIMIT_EXCEEDED", "Use the size limits defined in the v1 schema.", parts)
			}
		}
	}
	visit(root)
	result := make([]domain.Diagnostic, 0, len(unique))
	for item := range unique {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
	return result
}

func (v *Validator) containsValue(location string) string {
	_, fragment, _ := strings.Cut(location, "#/")
	var value any = v.document
	for _, part := range strings.Split(fragment, "/") {
		switch node := value.(type) {
		case map[string]any:
			value = node[part]
		case []any:
			var index int
			if _, err := fmt.Sscan(part, &index); err != nil || index < 0 || index >= len(node) {
				return "required by the v1 schema"
			}
			value = node[index]
		default:
			return "required by the v1 schema"
		}
	}
	if node, ok := value.(map[string]any); ok {
		if contains, ok := node["contains"].(map[string]any); ok {
			value = contains
		}
	}
	if node, ok := value.(map[string]any); ok {
		if expected, ok := node["const"].(string); ok {
			return expected
		}
	}
	return "required by the v1 schema"
}
