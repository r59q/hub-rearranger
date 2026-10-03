package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func TestFailureEvidenceRetainsSafeCodeWithoutRawError(t *testing.T) {
	for _, item := range []struct {
		name string
		err  error
		code string
	}{
		{"known", domain.HistoryUnavailable, "REPLAY_STATE_UNAVAILABLE"},
		{"wrapped", fmt.Errorf("private source: %w", domain.ProtectedChange), "PROTECTED_CHANGE"},
		{"unknown", errors.New("token=secret; private source"), "GITHUB_UNAVAILABLE"},
		{"unrecognized code", domain.Failure("token=secret"), "GITHUB_UNAVAILABLE"},
	} {
		t.Run(item.name, func(t *testing.T) {
			t.Setenv("GITHUB_RUN_ID", "100")
			t.Setenv("GITHUB_RUN_ATTEMPT", "2")
			path := filepath.Join(t.TempDir(), "publication-status.json")

			err := writeFailure(path, failureCode(item.err))

			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var actual map[string]any
			if json.Unmarshal(data, &actual) != nil || len(actual) != 5 || actual["reason_code"] != item.code || actual["outcome"] != "incomplete" || actual["run_id"] != float64(100) || actual["run_attempt"] != float64(2) || actual["version"] != float64(1) {
				t.Fatal("failure artifact lost identity or exposed unsafe data")
			}
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatal("failure artifact permissions")
			}
		})
	}
}

func TestFailureEvidenceRejectsInvalidRunIdentity(t *testing.T) {
	t.Setenv("GITHUB_RUN_ID", "100")
	t.Setenv("GITHUB_RUN_ATTEMPT", "invalid")
	path := filepath.Join(t.TempDir(), "publication-status.json")

	err := writeFailure(path, failureCode(domain.Invalid))

	if !errors.Is(err, domain.Invalid) {
		t.Fatal("invalid run identity accepted")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unbound failure artifact written")
	}
}
