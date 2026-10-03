package profiles

import (
	"bytes"
	"reflect"

	"go.yaml.in/yaml/v3"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

// MergeBootstrapCatalog preserves a matching profile byte-for-byte. Adding the
// profile preserves other entries and YAML comments; only that catalog is encoded.
// An existing policy is never silently enabled or replaced by a bootstrap plan.
func (v *Validator) MergeBootstrapCatalog(existing, proposed []byte) ([]byte, []domain.Diagnostic) {
	canonical, diagnostics := v.Validate(proposed)
	if len(diagnostics) > 0 {
		return nil, diagnostics
	}
	if len(existing) == 0 {
		return proposed, nil
	}
	current, diagnostics := v.Validate(existing)
	if len(diagnostics) > 0 {
		return nil, diagnostics
	}
	if profile, exists := current["codex-thorough"]; exists {
		if !reflect.DeepEqual(profile, canonical["codex-thorough"]) {
			return nil, []domain.Diagnostic{{Code: "PROFILE_CONFLICT", Message: "Review the existing codex-thorough policy; bootstrap will not replace or enable it."}}
		}
		return existing, nil
	}

	var target, source yaml.Node
	if yaml.Unmarshal(existing, &target) != nil || yaml.Unmarshal(proposed, &source) != nil {
		return nil, []domain.Diagnostic{*invalidYAML()}
	}
	targetProfiles, sourceProfiles := profileMapping(&target), profileMapping(&source)
	targetProfiles.Content = append(targetProfiles.Content, sourceProfiles.Content...)
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if encoder.Encode(&target) != nil || encoder.Close() != nil {
		return nil, []domain.Diagnostic{*invalidYAML()}
	}
	_, diagnostics = v.Validate(buffer.Bytes())
	return buffer.Bytes(), diagnostics
}

func profileMapping(document *yaml.Node) *yaml.Node {
	root := document.Content[0]
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "profiles" {
			return root.Content[i+1]
		}
	}
	return nil // A validated v1 catalog always has a profiles mapping.
}
