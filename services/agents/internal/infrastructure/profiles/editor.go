package profiles

import (
	"encoding/json"
	"reflect"
	"sort"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

func (v *Validator) editorProfile(existing, canonical []byte) (map[string]any, []domain.Diagnostic) {
	defaults, issues := v.Validate(canonical)
	if len(issues) > 0 || defaults["codex-thorough"] == nil {
		return nil, []domain.Diagnostic{{Code: "PROFILE_CONFLICT", Path: "/", Message: "Restore the reviewed canonical codex-thorough package."}}
	}
	profile := defaults["codex-thorough"]
	if len(existing) == 0 {
		return profile, issues
	}
	current, issues := v.Validate(existing)
	if len(issues) > 0 {
		return nil, issues
	}
	if selected := current["codex-thorough"]; selected != nil {
		for _, field := range []string{"role", "adapter", "model", "triggers", "authority", "validation"} {
			if !reflect.DeepEqual(profile[field], selected[field]) {
				return nil, []domain.Diagnostic{{Code: "PROFILE_CONFLICT", Path: "/profiles/<profile>/" + field, Message: "The installed adapter requires its canonical policy. Review this existing policy on GitHub before using the editor."}}
			}
		}
		profile = selected
	}
	return profile, issues
}

func (v *Validator) ReadProfileDraft(existing, canonical []byte) (domain.ProfileDraft, []domain.Diagnostic) {
	profile, issues := v.editorProfile(existing, canonical)
	if len(issues) > 0 {
		return domain.ProfileDraft{}, issues
	}
	context := profile["context"].(map[string]any)
	sources := []string{}
	for _, source := range context["sources"].([]any) {
		sources = append(sources, source.(string))
	}
	sort.Strings(sources)
	return domain.ProfileDraft{Name: profile["name"].(string), Description: profile["description"].(string), Enabled: profile["enabled"].(bool), ContextSources: sources, ReviewComments: profile["continuation"].(map[string]any)["review_comments"].(bool)}, issues
}

func (v *Validator) MergeProfileDraft(existing, canonical []byte, draft domain.ProfileDraft) ([]byte, []domain.Diagnostic) {
	profile, issues := v.editorProfile(existing, canonical)
	if len(issues) > 0 {
		return nil, issues
	}
	profile["name"], profile["description"], profile["enabled"] = draft.Name, draft.Description, draft.Enabled
	// Sorting makes reordered checkbox submissions produce the same reviewed bytes.
	sources := append([]string{}, draft.ContextSources...)
	sort.Strings(sources)
	profile["context"] = map[string]any{"sources": sources, "images": false}
	profile["continuation"] = map[string]any{"review_comments": draft.ReviewComments, "pipeline": "disabled"}
	encoded, err := json.Marshal(map[string]any{"schema_version": 1, "profiles": map[string]any{"codex-thorough": profile}})
	if err != nil {
		return nil, []domain.Diagnostic{*invalidYAML()}
	}
	if _, issues := v.Validate(encoded); len(issues) > 0 {
		return nil, issues
	}
	merged, err := mergeEditorCatalog(existing, encoded)
	if err != nil {
		return nil, []domain.Diagnostic{*invalidYAML()}
	}
	if _, issues := v.Validate(merged); len(issues) > 0 {
		return nil, issues
	}
	if len(existing) > 0 && v.sameEditorPolicy(existing, merged) {
		return existing, []domain.Diagnostic{}
	}
	return merged, []domain.Diagnostic{}
}

func (v *Validator) sameEditorPolicy(existing, proposed []byte) bool {
	before, _ := v.Validate(existing)
	after, _ := v.Validate(proposed)
	for _, catalog := range []map[string]map[string]any{before, after} {
		if selected := catalog["codex-thorough"]; selected != nil {
			sources := selected["context"].(map[string]any)["sources"].([]any)
			sort.Slice(sources, func(i, j int) bool { return sources[i].(string) < sources[j].(string) })
		}
	}
	return reflect.DeepEqual(before, after)
}
