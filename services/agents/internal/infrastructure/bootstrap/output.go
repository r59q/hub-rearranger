package bootstrap

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

var ErrConflicts = errors.New("resolve bootstrap conflicts before exporting the package")

// SeparateOutput prevents staging inside either checkout, including through a
// symlinked parent. Output's parent must already exist; its directory must be new.
func SeparateOutput(directory string, checkouts ...string) bool {
	parent, err := filepath.EvalSymlinks(filepath.Dir(directory))
	if err != nil {
		return false
	}
	output, err := filepath.Abs(filepath.Join(parent, filepath.Base(directory)))
	if err != nil {
		return false
	}
	for _, checkout := range checkouts {
		if checkout == "" {
			continue
		}
		resolved, err := filepath.EvalSymlinks(checkout)
		if err != nil {
			return false
		}
		absolute, err := filepath.Abs(resolved)
		if err != nil {
			return false
		}
		relative, err := filepath.Rel(absolute, output)
		if err != nil || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
			return false
		}
	}
	return true
}

// WritePackage only creates a new staging directory. It cannot edit the target
// checkout or reuse an existing output, even when generation is idempotent.
func WritePackage(directory string, plan domain.BootstrapPlan) error {
	if len(plan.Diagnostics) > 0 {
		return ErrConflicts
	}
	if err := os.Mkdir(directory, 0o700); err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(directory)
		}
	}()
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, file := range plan.Files {
		if !fs.ValidPath(file.Path) || file.Path == "." || strings.Contains(file.Path, "\\") {
			return ErrUnsafeFile
		}
		if err := root.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
			return err
		}
		if err := root.WriteFile(file.Path, []byte(file.Content), 0o644); err != nil {
			return err
		}
	}
	complete = true
	return nil
}

// Diff uses Git's established patch renderer, with repository config, external
// diff drivers and text conversion disabled. It never runs proposed source.
func Diff(ctx context.Context, plan domain.BootstrapPlan, existing map[string]string) ([]byte, error) {
	directory, err := os.MkdirTemp("", "hub-bootstrap-diff-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(directory)
	before := domain.BootstrapPlan{}
	after := domain.BootstrapPlan{}
	for _, file := range plan.Files {
		if file.Status == "conflict" {
			continue
		}
		after.Files = append(after.Files, file)
		if content, exists := existing[file.Path]; exists {
			before.Files = append(before.Files, domain.BootstrapFile{Path: file.Path, Content: content})
		}
	}
	if err := WritePackage(filepath.Join(directory, "before"), before); err != nil {
		return nil, err
	}
	if err := WritePackage(filepath.Join(directory, "after"), after); err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, "git", "diff", "--no-index", "--no-ext-diff", "--no-textconv",
		"--src-prefix=a/", "--dst-prefix=b/", "--", "before", "after")
	command.Dir = directory
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + directory,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "LC_ALL=C"}
	output, err := command.Output()
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		return nil, errors.New("Git could not render the bootstrap diff")
	}
	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "diff --git ") || strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			lines[i] = strings.ReplaceAll(strings.ReplaceAll(line, "a/before/", "a/"), "b/after/", "b/")
			// Git uses the after path on both sides of an added file header.
			lines[i] = strings.ReplaceAll(lines[i], "a/after/", "a/")
		}
	}
	return []byte(strings.Join(lines, "\n")), nil
}
