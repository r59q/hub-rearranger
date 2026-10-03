package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func failureCode(err error) string {
	var failure domain.Failure
	if errors.As(err, &failure) {
		switch failure {
		case domain.Invalid, domain.Denied, domain.SourceChanged, domain.ProfileInvalid,
			domain.ProfileDisabled, domain.ProfileChanged, domain.RevisionInvalid,
			domain.Unavailable, domain.HistoryUnavailable, domain.StaleHead, domain.ProtectedChange:
			return string(failure)
		}
	}
	return string(domain.Unavailable)
}

// Failure evidence retains only a fixed code and trusted Actions run identity.
// Raw errors may contain API credentials or source and must never be serialized.
func writeFailure(path, code string) error {
	run, runErr := strconv.ParseInt(os.Getenv("GITHUB_RUN_ID"), 10, 64)
	attempt, attemptErr := strconv.Atoi(os.Getenv("GITHUB_RUN_ATTEMPT"))
	if !filepath.IsAbs(path) || runErr != nil || attemptErr != nil || run <= 0 || attempt <= 0 {
		return domain.Invalid
	}
	record := struct {
		Version    int    `json:"version"`
		RunID      int64  `json:"run_id"`
		RunAttempt int    `json:"run_attempt"`
		Outcome    string `json:"outcome"`
		ReasonCode string `json:"reason_code"`
	}{1, run, attempt, "incomplete", code}
	data, err := json.Marshal(record)
	if err != nil || os.WriteFile(path, data, 0600) != nil {
		return domain.Unavailable
	}
	return nil
}
