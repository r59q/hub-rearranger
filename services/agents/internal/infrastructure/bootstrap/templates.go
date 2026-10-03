package bootstrap

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

var ErrTemplates = errors.New("canonical bootstrap package is incomplete or unsupported; use a reviewed checkout")

var packageFiles = []string{
	domain.ProfileCatalogPath,
	".github/workflows/agent-assignment.yml",
	".github/workflows/agent-profile-diagnostic.yml",
	".github/workflows/agent-bootstrap-checks.yml",
	".github/actionlint.yaml",
	"docs/agent-workflows.md",
	"ops/agent-intake/go.mod", "ops/agent-intake/go.sum",
	"ops/agent-intake/requirements-dev.txt", "ops/agent-intake/README.md", "ops/agent-intake/.gitignore",
	"ops/agent-profiles/schema.v1.json", "ops/agent-profiles/requirements-dev.txt",
	"ops/agent-profiles/README.md", "ops/agent-profiles/.gitignore",
	"ops/private-runner/readiness.schema.v1.json", "ops/private-runner/result.schema.v1.json",
	"ops/private-runner/requirements-dev.txt", "ops/private-runner/pyproject.toml",
	"ops/private-runner/README.md", "ops/private-runner/READINESS.md",
	"ops/private-runner/EXECUTION.md", "ops/private-runner/.gitignore",
	"ops/private-runner/authorize.mjs", "ops/private-runner/test_authorize.mjs",
}

// Templates reads canonical sources on each invocation. Only repository trust
// exceptions are narrowed in generated workflows; adapter logic stays canonical.
func Templates(root *os.Root) (map[string]string, error) {
	files := map[string]string{"AGENTS.md": domain.BootstrapInstructions}
	paths := append([]string{}, packageFiles...)
	for _, directory := range []string{"ops/agent-intake", "ops/agent-profiles", "ops/private-runner"} {
		err := fs.WalkDir(root.FS(), directory, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if path != directory && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "__pycache__") {
					return fs.SkipDir
				}
				return nil
			}
			// Exporting Hub's API schemas and the historical addons diagnostic
			// tests are separate concerns, with dependencies outside this package.
			if entry.Name() == "export_contract.py" || entry.Name() == "test_diagnose.py" {
				return nil
			}
			if strings.HasSuffix(path, ".py") || (directory == "ops/agent-intake" && strings.HasSuffix(path, ".go")) {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, ErrTemplates
		}
	}
	for _, path := range paths {
		content, err := ReadFile(root, path)
		if err != nil || content == "" {
			return nil, ErrTemplates
		}
		files[path] = content
	}
	for _, path := range []string{".github/workflows/agent-assignment.yml", ".github/workflows/agent-profile-diagnostic.yml"} {
		content, err := privateWorkflow(files[path])
		if err != nil {
			return nil, err
		}
		files[path] = content
	}
	return files, nil
}

func privateWorkflow(source string) (string, error) {
	const positive = "(github.event.repository.private == true || github.repository == 'r59q/hub-rearranger')"
	const negative = "(github.event.repository.private != true && github.repository != 'r59q/hub-rearranger')"
	if !strings.Contains(source, positive) {
		return "", ErrTemplates
	}
	source = strings.ReplaceAll(source, positive, "github.event.repository.private == true")
	source = strings.ReplaceAll(source, negative, "github.event.repository.private != true")
	source = strings.ReplaceAll(source,
		"# Private runners by default; the operator approved addons for r59q/hub-rearranger.",
		"# Private repositories only; install a dedicated restricted runner before enabling.")
	if strings.Contains(source, "r59q/hub-rearranger") || strings.Contains(source, "addons") {
		return "", ErrTemplates
	}
	return source, nil
}
