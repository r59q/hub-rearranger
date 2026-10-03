// Package patch applies proposals using reviewed Git/index code with no token.
package patch

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

type Python struct{ Executable, Script string }

func (p Python) Apply(ctx context.Context, archive, patch []byte) ([]domain.FileChange, error) {
	directory, err := os.MkdirTemp("", "agent-publish-transfer-")
	if err != nil {
		return nil, domain.Unavailable
	}
	defer func() { _ = os.RemoveAll(directory) }()
	if os.WriteFile(filepath.Join(directory, "source.tar.gz"), archive, 0600) != nil || os.WriteFile(filepath.Join(directory, "proposal.patch"), patch, 0600) != nil {
		return nil, domain.Unavailable
	}
	output := filepath.Join(directory, "changes.json")
	command := exec.CommandContext(ctx, p.Executable, "-I", p.Script, "--source", filepath.Join(directory, "source.tar.gz"), "--patch", filepath.Join(directory, "proposal.patch"), "--output", output)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8"}
	if command.Run() != nil {
		return nil, domain.ProtectedChange
	}
	data, err := os.ReadFile(output)
	if err != nil || len(data) > 180*1024*1024 {
		return nil, domain.ProtectedChange
	}
	var changes []domain.FileChange
	if json.Unmarshal(data, &changes) != nil {
		return nil, domain.ProtectedChange
	}
	return changes, nil
}
