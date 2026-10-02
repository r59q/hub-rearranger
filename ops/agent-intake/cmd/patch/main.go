// Command patch reconstructs authorization and prepares private runner inputs.
// It never invokes Codex or writes to GitHub.
package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/infrastructure/event"
	provider "github.com/r59q/hub-rearranger/ops/agent-intake/internal/infrastructure/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/infrastructure/policy"
)

func write(path string, value []byte) error {
	if err := os.WriteFile(path, value, 0600); err != nil {
		return domain.Unavailable
	}
	return nil
}

func validate(ctx context.Context, python, script string, request map[string]any) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return domain.Invalid
	}
	command := exec.CommandContext(ctx, python, "-I", script)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8"}
	command.Stdin = bytes.NewReader(payload)
	command.Stdout, command.Stderr = io.Discard, io.Discard
	if command.Run() != nil {
		return domain.Failure("RUNNER_NOT_READY")
	}
	return nil
}

func prepare(ctx context.Context, python, policyScript, evidenceScript, directory string, source bool) error {
	payload, err := os.ReadFile(os.Getenv("GITHUB_EVENT_PATH"))
	if err != nil || len(payload) > 2*1024*1024 {
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
	client := provider.New(gh.NewClient(&http.Client{Timeout: 15 * time.Second}).WithAuthToken(os.Getenv("GH_TOKEN")))
	service := domain.New(client, policy.Python{Executable: python, Script: policyScript})
	input, err := client.ExecutionInput(ctx, request)
	if err != nil {
		return err
	}
	input, err = service.Reauthorize(ctx, request, input)
	if err != nil {
		return err
	}
	readiness, err := client.ReadinessRecord(ctx, input)
	if err != nil {
		return err
	}
	previous, artifactID, err := client.PriorExecution(ctx, input)
	if err != nil {
		return err
	}
	verification := map[string]any{"invocation": input, "readiness": json.RawMessage(readiness)}
	if previous != nil {
		if len(previous) != 3 || previous["result.json"] == nil || previous["proposal.patch"] == nil || previous["summary.json"] == nil {
			return domain.HistoryUnavailable
		}
		verification["previous"] = map[string]any{"result": json.RawMessage(previous["result.json"]), "patch_hex": hex.EncodeToString(previous["proposal.patch"]), "summary_hex": hex.EncodeToString(previous["summary.json"])}
	}
	if err := validate(ctx, python, evidenceScript, verification); err != nil {
		return err
	}
	if previous != nil {
		// The write job will independently reauthorize and verify this immutable
		// artifact. Reuse never means publication succeeded.
		return outputs(false, artifactID)
	}
	if !source {
		return outputs(true, 0)
	}
	archive, err := client.ExecutionSource(ctx, input)
	if err != nil {
		return err
	}
	contextData, err := client.ExecutionContext(ctx, input)
	if err != nil {
		return err
	}
	// Reauthorize again after collection; the workload must not run on an old
	// access snapshot if roles, issue state, or current policy changed meanwhile.
	input, err = service.Reauthorize(ctx, request, input)
	if err != nil {
		return err
	}
	data, _ := json.Marshal(input)
	for name, content := range map[string][]byte{"invocation.json": data, "readiness.json": readiness, "source.tar.gz": archive, "context.json": contextData} {
		if err := write(filepath.Join(directory, name), content); err != nil {
			return err
		}
	}
	return nil
}

func outputs(run bool, artifactID int64) error {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return nil
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return domain.Unavailable
	}
	defer func() { _ = file.Close() }()
	_, err = fmt.Fprintf(file, "execute=%t\nrecovered_artifact_id=%d\n", run, artifactID)
	return err
}

func main() {
	python := flag.String("python", "", "Trusted validation Python")
	policyScript := flag.String("policy", "", "Trusted canonical profile adapter")
	evidenceScript := flag.String("evidence", "", "Trusted execution evidence validator")
	directory := flag.String("output", "", "Private input directory")
	source := flag.Bool("source", false, "Collect source for the installed launcher")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	var err error
	if !filepath.IsAbs(*python) || !filepath.IsAbs(*policyScript) || !filepath.IsAbs(*evidenceScript) || !filepath.IsAbs(*directory) || os.Getenv("GH_TOKEN") == "" {
		err = domain.Invalid
	} else if os.MkdirAll(*directory, 0700) != nil {
		err = domain.Unavailable
	} else {
		err = prepare(ctx, *python, *policyScript, *evidenceScript, *directory, *source)
	}
	if err != nil {
		// Neither API errors nor issue/source/provider content enters public logs.
		code := "RUNNER_NOT_READY"
		if failure, ok := err.(domain.Failure); ok {
			code = string(failure)
		}
		fmt.Fprintln(os.Stderr, code+": execution was not authorized; repair setup or rerun the original assignment after verification.")
		os.Exit(1)
	}
}
