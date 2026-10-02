package report

import (
	"errors"
	"strings"
	"testing"

	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func TestReportsOnlyFixedRecoveryText(t *testing.T) {
	for _, err := range []error{errors.New("private-sentinel"), domain.Failure("private-sentinel"), domain.ProfileChanged, domain.Denied} {
		result := Rejected(err)

		if strings.Contains(result, "private-sentinel") || !strings.Contains(result, "Agent assignment rejected") {
			t.Fatal("unsafe report")
		}
	}
}
