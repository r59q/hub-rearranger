package bootstrap

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/profiles"
)

func canonicalTemplates(t *testing.T) map[string]string {
	t.Helper()
	root, err := os.OpenRoot("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	files, err := Templates(root)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestPackageRetainsCanonicalWorkflowsAndNarrowsPublicExceptions(t *testing.T) {
	// Arrange.
	files := canonicalTemplates(t)

	// Act.
	for _, path := range []string{".github/workflows/agent-assignment.yml", ".github/workflows/agent-profile-diagnostic.yml"} {
		source, err := os.ReadFile("../../../../../" + path)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := privateWorkflow(string(source))
		if err != nil || files[path] != expected {
			t.Fatal("workflow diverged from canonical implementation")
		}
	}

	// Assert.
	assignment, ok := profiles.ParseDocument([]byte(files[".github/workflows/agent-assignment.yml"]))
	if !ok {
		t.Fatal("assignment is not valid bounded workflow YAML")
	}
	jobs := assignment.(map[string]any)["jobs"].(map[string]any)
	for _, name := range []string{"patch", "publish"} {
		guard := jobs[name].(map[string]any)["if"].(string)
		if !strings.Contains(guard, "github.event.repository.private == true") || strings.Contains(guard, "github.repository ==") {
			t.Fatal("generated workflow retained public exception")
		}
	}
	for path, content := range files {
		if strings.Contains(path, ".env") || strings.Contains(path, "auth.json") || strings.Contains(path, "operator.json") || strings.Contains(path, ".venv") {
			t.Fatal("local runtime state entered the package")
		}
		if strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".py") || strings.HasSuffix(path, ".json") {
			source, err := os.ReadFile("../../../../../" + path)
			if err != nil || content != string(source) {
				t.Fatalf("helper was not canonical: %s", path)
			}
		}
	}
	for _, filename := range []string{"diagnose.py", "profile_diagnostic.py"} {
		digest := fmt.Sprintf("%x  %s", sha256.Sum256([]byte(files["ops/private-runner/"+filename])), filename)
		if !strings.Contains(files[".github/workflows/agent-profile-diagnostic.yml"], digest) {
			t.Fatal("probe checksum does not match packaged script")
		}
	}
	launcherDigest := fmt.Sprintf("%x  execution.py", sha256.Sum256([]byte(files["ops/private-runner/execution.py"])))
	if !strings.Contains(files[".github/workflows/agent-assignment.yml"], launcherDigest) {
		t.Fatal("launcher checksum does not match packaged script")
	}
	for _, path := range []string{"ops/agent-intake/cmd/intake/main.go", "ops/agent-intake/cmd/patch/main.go", "ops/agent-intake/cmd/publish/main.go", "ops/agent-profiles/schema.v1.json", "ops/private-runner/readiness.schema.v1.json", "docs/agent-workflows.md", ".github/workflows/agent-bootstrap-checks.yml"} {
		if files[path] == "" {
			t.Fatalf("missing required installation artifact: %s", path)
		}
	}
}

func TestExportedPackageConvergesWithoutTouchingTargetFiles(t *testing.T) {
	// Arrange.
	templates := canonicalTemplates(t)
	validator, err := profiles.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	plan := domain.PlanBootstrap(templates, nil, validator)
	directory := t.TempDir() + "/package"

	// Act.
	if err := WritePackage(directory, plan); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	snapshot, err := Snapshot(root, templates)
	if err != nil {
		t.Fatal(err)
	}
	second := domain.PlanBootstrap(templates, snapshot, validator)

	// Assert.
	if len(second.Diagnostics) != 0 {
		t.Fatal(second.Diagnostics)
	}
	for _, file := range second.Files {
		if file.Status != "unchanged" {
			t.Fatalf("second generation modified %s", file.Path)
		}
	}
	if err := WritePackage(directory, plan); err == nil {
		t.Fatal("export overwrote an existing target")
	}
}
