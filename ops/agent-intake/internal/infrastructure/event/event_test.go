package event

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func TestEventUsesOnlyCreatedHumanCommentsInTheWorkflowRepository(t *testing.T) {
	environment := map[string]string{"GITHUB_EVENT_NAME": "issue_comment", "GITHUB_SERVER_URL": "https://github.com", "GITHUB_RUN_ID": "100", "GITHUB_RUN_ATTEMPT": "1",
		"GITHUB_REPOSITORY": "octo/demo", "GITHUB_ACTOR": "maintainer", "GITHUB_TRIGGERING_ACTOR": "maintainer", "GITHUB_WORKFLOW_SHA": strings.Repeat("1", 40)}
	for _, variant := range []string{"valid", "edited", "bot", "sender", "other-repository", "pull-request-event", "bad-id", "enterprise", "malformed"} {
		t.Run(variant, func(t *testing.T) {
			e := map[string]string{}
			for key, value := range environment {
				e[key] = value
			}
			payload := map[string]any{"action": "created", "repository": map[string]any{"id": 42, "name": "demo", "full_name": "octo/demo", "owner": map[string]any{"login": "octo"}},
				"issue": map[string]any{"number": 3}, "comment": map[string]any{"id": 99, "body": "/agent assign safe", "user": map[string]any{"id": 7, "login": "maintainer", "type": "User"}}, "sender": map[string]any{"id": 7}}
			switch variant {
			case "edited":
				payload["action"] = "edited"
			case "bot":
				payload["comment"].(map[string]any)["user"].(map[string]any)["type"] = "Bot"
			case "sender":
				payload["sender"].(map[string]any)["id"] = 8
			case "other-repository":
				e["GITHUB_REPOSITORY"] = "other/demo"
			case "pull-request-event":
				e["GITHUB_EVENT_NAME"] = "pull_request"
			case "bad-id":
				e["GITHUB_RUN_ID"] = "unverified"
			case "enterprise":
				e["GITHUB_SERVER_URL"] = "https://other.example"
			}
			data, _ := json.Marshal(payload)
			if variant == "malformed" {
				data = []byte("private-sentinel")
			}

			result, err := Parse(data, e)

			if variant == "valid" {
				if err != nil || result.CommentID != 99 || result.Repository.ID != 42 || result.Requester.ID != 7 {
					t.Fatal("verified event rejected")
				}
			} else if !errors.Is(err, domain.Invalid) {
				t.Fatal("unverified event accepted")
			}
		})
	}
}
