package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

func TestCommandExportsACompletePackageAndThenReportsNoChanges(t *testing.T) {
	// Arrange.
	directory := t.TempDir() + "/package"
	var stdout, stderr bytes.Buffer

	// Act.
	code := run([]string{"--source", "../../../..", "--output", directory}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("export failed: %d %s", code, &stderr)
	}
	first := append([]byte{}, stdout.Bytes()...)
	stdout.Reset()
	code = run([]string{"--source", "../../../..", "--target", directory}, &stdout, &stderr)
	var second domain.BootstrapPlan
	err := json.Unmarshal(stdout.Bytes(), &second)

	// Assert.
	if code != 0 || err != nil || len(second.Files) == 0 || len(second.Diagnostics) != 0 {
		t.Fatalf("comparison failed: %d %v %s", code, err, &stderr)
	}
	for _, file := range second.Files {
		if file.Status != "unchanged" {
			t.Fatalf("export did not converge: %s", file.Path)
		}
	}
	stdout.Reset()
	code = run([]string{"--source", "../../../.."}, &stdout, &stderr)
	if code != 0 || !bytes.Equal(first, stdout.Bytes()) {
		t.Fatal("JSON output depends on output paths or wall clock")
	}
}

func TestCommandReturnsAConflictWithoutCreatingOutputOrModifyingPolicy(t *testing.T) {
	// Arrange.
	target := t.TempDir()
	if err := os.Mkdir(target+"/.github", 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("schema_version: 2\nprofiles: {}\n")
	if err := os.WriteFile(target+"/.github/agent-profiles.yml", content, 0o644); err != nil {
		t.Fatal(err)
	}
	output := t.TempDir() + "/package"
	var stdout, stderr bytes.Buffer

	// Act.
	code := run([]string{"--source", "../../../..", "--target", target, "--output", output}, &stdout, &stderr)
	after, err := os.ReadFile(target + "/.github/agent-profiles.yml")

	// Assert.
	if code != 2 || err != nil || !bytes.Equal(after, content) || !strings.Contains(stderr.String(), "UNSUPPORTED_VERSION") {
		t.Fatalf("conflict changed target or lacked safe guidance: %d %s", code, &stderr)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("conflicted command exported a partial installation")
	}
}
