package domain

import "strings"

const BootstrapInstructions = `<!-- hub-agent-bootstrap:v1:begin -->
## GitHub-native agent assignments

Use the repository-local codex-thorough profile and the GitHub issue-comment
convention documented in docs/agent-workflows.md. GitHub remains the source of
truth; Hub is optional. Assignment authority is limited to a dedicated branch
and draft PR. Never merge, change workflows/profile/runtime tooling, modify
instructions, or access credentials as part of an implementation assignment.
Keep runner authentication outside checkouts and artifacts. Workload commands
must retain the verified filesystem isolation and disabled network policy.
Run the repository's fixed make check validation and report failures honestly;
a published draft or green Actions run does not imply validation passed.
<!-- hub-agent-bootstrap:v1:end -->
`

func mergeBootstrapInstructions(existing, proposed string) (string, []Diagnostic) {
	const begin = "<!-- hub-agent-bootstrap:v1:begin -->"
	const end = "<!-- hub-agent-bootstrap:v1:end -->"
	starts, ends := strings.Count(existing, begin), strings.Count(existing, end)
	if starts == 0 && ends == 0 {
		if existing == "" {
			return proposed, nil
		}
		separator := "\n"
		if !strings.HasSuffix(existing, "\n") {
			separator += "\n"
		}
		return existing + separator + proposed, nil
	}
	start, finish := strings.Index(existing, begin), strings.Index(existing, end)
	if starts != 1 || ends != 1 || finish < start {
		return "", []Diagnostic{{Code: "INSTRUCTIONS_CONFLICT", Message: "Repair the single bootstrap instruction marker pair before generating again."}}
	}
	finish += len(end)
	if strings.HasPrefix(existing[finish:], "\n") {
		finish++
	}
	return existing[:start] + proposed + existing[finish:], nil
}
