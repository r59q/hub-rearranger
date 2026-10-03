// Command publish runs only in the separate GitHub-hosted write job.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/infrastructure/event"
	provider "github.com/r59q/hub-rearranger/ops/agent-intake/internal/infrastructure/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/infrastructure/patch"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/infrastructure/policy"
)

func appendFile(path, value string) error {
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return domain.Unavailable
	}
	defer func() { _ = f.Close() }()
	if _, err = f.WriteString(value); err != nil {
		return domain.Unavailable
	}
	return nil
}

func execute() error {
	python := flag.String("python", "", "Trusted validation Python")
	catalog := flag.String("policy", "", "Canonical profile validator")
	evidence := flag.String("evidence", "", "Canonical publication evidence validator")
	patchScript := flag.String("patch", "", "Trusted cached patch adapter")
	output := flag.String("output", "", "Safe publication result")
	flag.Parse()
	for _, path := range []string{*python, *catalog, *evidence, *patchScript, *output} {
		if !filepath.IsAbs(path) {
			return domain.Invalid
		}
	}
	if os.Getenv("GH_TOKEN") == "" {
		return domain.Invalid
	}
	file, err := os.Open(os.Getenv("GITHUB_EVENT_PATH"))
	if err != nil {
		return domain.Invalid
	}
	defer func() { _ = file.Close() }()
	payload, err := io.ReadAll(io.LimitReader(file, 2*1024*1024+1))
	if err != nil {
		return domain.Invalid
	}
	environment := map[string]string{}
	for _, key := range []string{"GITHUB_EVENT_NAME", "GITHUB_SERVER_URL", "GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT", "GITHUB_REPOSITORY", "GITHUB_ACTOR", "GITHUB_TRIGGERING_ACTOR", "GITHUB_WORKFLOW_SHA"} {
		environment[key] = os.Getenv(key)
	}
	request, err := event.Parse(payload, environment)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	client := provider.New(gh.NewClient(&http.Client{Timeout: 15 * time.Second}).WithAuthToken(os.Getenv("GH_TOKEN")))
	authorization := domain.New(client, policy.Python{Executable: *python, Script: *catalog})
	input, err := client.ExecutionInput(ctx, request)
	if err != nil {
		return err
	}
	publisher := domain.NewPublisher(authorization, client, client, policy.Proposal{Executable: *python, Script: *evidence}, patch.Python{Executable: *python, Script: *patchScript})
	publication, err := publisher.Publish(ctx, request, input)
	if err != nil {
		return err
	}
	record := map[string]any{"version": 1, "assignment_id": input.AssignmentID, "run_id": input.RunID, "run_attempt": input.RunAttempt, "outcome": "published", "reason_code": "DRAFT_PR_PUBLISHED", "publication": publication}
	data, _ := json.Marshal(record)
	if os.WriteFile(*output, data, 0600) != nil {
		return domain.Unavailable
	}
	if err = appendFile(os.Getenv("GITHUB_OUTPUT"), fmt.Sprintf("outcome=published\npr_number=%d\n", publication.PRNumber)); err != nil {
		return err
	}
	return appendFile(os.Getenv("GITHUB_STEP_SUMMARY"), fmt.Sprintf("## Agent publication\n\nDraft [PR #%d](%s) and its verified publication evidence are available. Review repository validation on the PR; publication does not mean validation passed.\n", publication.PRNumber, publication.PRURL))
}

func main() {
	if err := execute(); err != nil {
		code := "GITHUB_UNAVAILABLE"
		if failure, ok := err.(domain.Failure); ok {
			code = string(failure)
		}
		message := code + ": publication incomplete. Review the original run and reconcile its branch, PR and evidence before retrying.\n"
		_ = appendFile(os.Getenv("GITHUB_STEP_SUMMARY"), message)
		fmt.Fprint(os.Stderr, message)
		os.Exit(1)
	}
}
