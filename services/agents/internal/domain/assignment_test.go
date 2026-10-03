package domain

import (
	"context"
	"strings"
	"testing"
)

type assignmentReaderStub struct{ issue IssueAssignment }

func (r assignmentReaderStub) ReadAssignment(context.Context, Repository, int64) (IssueAssignment, error) {
	return r.issue, nil
}

func TestIssueAssignmentRequiresEnabledProfileOpenSourceAndReviewOfExistingRequest(t *testing.T) {
	for _, variant := range []string{"ready", "disabled", "closed", "locked", "invalid", "existing"} {
		t.Run(variant, func(t *testing.T) {
			reader := &catalogReader{file: CatalogFile{DefaultBranch: "main", Revision: strings.Repeat("a", 40)}}
			validator := &catalogValidator{profiles: map[string]map[string]any{"codex-thorough": {"enabled": variant != "disabled"}}, diagnostics: []Diagnostic{}}
			issue := IssueAssignment{Repository: "octo/demo", RepositoryID: 42, Number: 3, IssueState: "open", Requests: []AssignmentRequest{}}
			switch variant {
			case "closed":
				issue.IssueState = "closed"
			case "locked":
				issue.IssueLocked = true
			case "invalid":
				validator.diagnostics = []Diagnostic{{Code: "INVALID_YAML"}}
			case "existing":
				issue.Requests = []AssignmentRequest{{CommentID: 99}}
			}
			service := NewService(NewProfileService(reader, validator)).WithAssignments(assignmentReaderStub{issue})
			result, err := service.IssueAssignment(context.Background(), Repository{"octo", "demo"}, 3)
			if err != nil || result.Assignable != (variant == "ready") || result.Command != "/agent assign codex-thorough@"+reader.file.Revision+" authority=branch-draft-pr" {
				t.Fatal(result, err)
			}
		})
	}
}
