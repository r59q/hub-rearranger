// Command intake runs only in the trusted GitHub-hosted issue-comment job.
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
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/infrastructure/policy"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/report"
)

func appendFile(path, value string) error {
	if path == "" {
		return nil
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	_, err = file.WriteString(value)
	return err
}

func execute() error {
	python := flag.String("python", "", "Absolute path to the pinned profile-validation Python")
	script := flag.String("policy", "", "Absolute path to trusted profile_policy.py")
	output := flag.String("output", "", "Verified invocation output path")
	flag.Parse()
	if !filepath.IsAbs(*python) || !filepath.IsAbs(*script) || !filepath.IsAbs(*output) || os.Getenv("GH_TOKEN") == "" {
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

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	client := provider.New(gh.NewClient(&http.Client{Timeout: 10 * time.Second}).WithAuthToken(os.Getenv("GH_TOKEN")))
	service := domain.New(client, policy.Python{Executable: *python, Script: *script})

	decision, err := service.Intake(ctx, request)
	if err != nil {
		return err
	}

	if decision.Invocation != nil {
		data, err := json.MarshalIndent(decision.Invocation, "", "  ")
		if err != nil || os.WriteFile(*output, data, 0600) != nil {
			return domain.Unavailable
		}
	}

	if err := appendFile(os.Getenv("GITHUB_STEP_SUMMARY"), report.Accepted(decision, request.Repository)); err != nil {
		return domain.Unavailable
	}

	return appendFile(os.Getenv("GITHUB_OUTPUT"), fmt.Sprintf("dispatch=%t\ndisposition=%s\ncanonical_run_id=%d\n", decision.Invocation != nil, decision.Disposition, decision.CanonicalRunID))
}

func main() {
	if err := execute(); err != nil {
		message := report.Rejected(err)
		_ = appendFile(os.Getenv("GITHUB_STEP_SUMMARY"), message)
		fmt.Fprint(os.Stderr, message)
		os.Exit(1)
	}
}
