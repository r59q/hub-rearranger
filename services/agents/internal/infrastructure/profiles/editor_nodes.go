package profiles

import (
	"bytes"
	"errors"
	"reflect"

	"go.yaml.in/yaml/v3"
)

var errEditorNodes = errors.New("profile catalog could not be merged")

// Only editable values are replaced; fixed policy and unrelated nodes stay intact.
func mergeEditorCatalog(existing, encoded []byte) ([]byte, error) {
	var proposed yaml.Node
	if yaml.Unmarshal(encoded, &proposed) != nil {
		return nil, errEditorNodes
	}
	// JSON and YAML are decoded into a normal YAML node tree; serialize as YAML
	// while preserving unrelated profiles and their comments in the original tree.
	clearJSONStyle(&proposed)
	var target yaml.Node
	if len(existing) == 0 {
		target = proposed
	} else {
		if yaml.Unmarshal(existing, &target) != nil {
			return nil, errEditorNodes
		}
		targetProfiles, proposedProfiles := profileMapping(&target), profileMapping(&proposed)
		selected := mappingValue(targetProfiles, "codex-thorough")
		if selected == nil {
			targetProfiles.Content = append(targetProfiles.Content, proposedProfiles.Content...)
		} else {
			updated := mappingValue(proposedProfiles, "codex-thorough")
			for _, field := range []string{"name", "description", "enabled", "context", "continuation"} {
				old, next := mappingValue(selected, field), mappingValue(updated, field)
				// Leave unchanged nodes intact, including their original ordering/comments.
				var oldValue, nextValue any
				if old.Decode(&oldValue) == nil && next.Decode(&nextValue) == nil && reflect.DeepEqual(oldValue, nextValue) {
					continue
				}
				next.HeadComment, next.LineComment, next.FootComment = old.HeadComment, old.LineComment, old.FootComment
				*old = *next
			}
		}
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if encoder.Encode(&target) != nil || encoder.Close() != nil {
		return nil, errEditorNodes
	}
	return output.Bytes(), nil
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func clearJSONStyle(node *yaml.Node) {
	node.Style = 0
	for _, child := range node.Content {
		clearJSONStyle(child)
	}
}
