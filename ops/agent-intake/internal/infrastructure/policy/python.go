// Package policy adapts the canonical AW-003 Python validator.
package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"

	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

type Python struct{ Executable, Script string }

func (p Python) Verify(ctx context.Context, pinned, current []byte, id string) (json.RawMessage, error) {
	request, _ := json.Marshal(struct {
		Pinned    []byte `json:"pinned"`
		Current   []byte `json:"current"`
		ProfileID string `json:"profile_id"`
	}{pinned, current, id})

	command := exec.CommandContext(ctx, p.Executable, "-I", p.Script)
	command.Stdin = bytes.NewReader(request)
	// Do not pass the workflow token or ambient Python configuration to validation.
	command.Env = []string{"LANG=C.UTF-8"}

	output, err := command.Output()
	if err != nil || len(output) > 16_384 {
		return nil, domain.ProfileInvalid
	}

	var response struct {
		Code    string          `json:"code"`
		Profile json.RawMessage `json:"profile"`
	}
	if json.Unmarshal(output, &response) != nil {
		return nil, domain.ProfileInvalid
	}

	switch response.Code {
	case "ACCEPTED":
		if len(response.Profile) == 0 {
			return nil, domain.ProfileInvalid
		}
		return response.Profile, nil
	case string(domain.ProfileDisabled):
		return nil, domain.ProfileDisabled
	case string(domain.ProfileChanged):
		return nil, domain.ProfileChanged
	default:
		return nil, domain.ProfileInvalid
	}
}
