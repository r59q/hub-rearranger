package policy

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"os/exec"

	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

type Proposal struct{ Executable, Script string }

func (p Proposal) VerifyProposal(ctx context.Context, input domain.Invocation, proposal domain.Proposal) error {
	payload, err := json.Marshal(map[string]any{"invocation": input, "origin": proposal.Invocation, "result": proposal.Result, "attempt": proposal.Attempt, "patch_hex": hex.EncodeToString(proposal.Patch), "summary_hex": hex.EncodeToString(proposal.Summary)})
	if err != nil || len(payload) > 26*1024*1024 {
		return domain.HistoryUnavailable
	}
	command := exec.CommandContext(ctx, p.Executable, "-I", p.Script)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8"}
	command.Stdin = bytes.NewReader(payload)
	output, err := command.Output()
	if err != nil || string(bytes.TrimSpace(output)) != `{"code":"VERIFIED"}` {
		return domain.HistoryUnavailable
	}
	return nil
}
