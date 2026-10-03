package bootstrap

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

func TestSnapshotRejectsSymlinkFilesParentsAndUnsupportedExistingContent(t *testing.T) {
	for _, variant := range []string{"symlink", "parent", "directory", "binary", "oversize"} {
		t.Run(variant, func(t *testing.T) {
			// Arrange.
			directory := t.TempDir()
			path := "file.txt"
			var err error
			switch variant {
			case "symlink":
				err = os.Symlink("missing", filepath.Join(directory, path))
			case "parent":
				err = os.Symlink(t.TempDir(), filepath.Join(directory, "nested"))
				path = "nested/file.txt"
			case "directory":
				err = os.Mkdir(filepath.Join(directory, path), 0o755)
			case "binary":
				err = os.WriteFile(filepath.Join(directory, path), []byte{0, 255}, 0o644)
			case "oversize":
				err = os.WriteFile(filepath.Join(directory, path), []byte(strings.Repeat("x", maxFileBytes+1)), 0o644)
			}
			if err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()

			// Act.
			_, err = Snapshot(root, map[string]string{path: "proposal"})

			// Assert.
			if !errors.Is(err, ErrUnsafeFile) {
				t.Fatalf("unsupported target was treated as an addition: %v", err)
			}
		})
	}
}

func TestConflictedExportAndEscapingPathsLeaveNoPackage(t *testing.T) {
	for _, plan := range []domain.BootstrapPlan{
		{Diagnostics: []domain.Diagnostic{{Code: "PROFILE_CONFLICT"}}},
		{Files: []domain.BootstrapFile{{Path: "../outside.txt", Content: "unsafe"}}},
	} {
		// Arrange.
		directory := t.TempDir() + "/package"

		// Act.
		err := WritePackage(directory, plan)

		// Assert.
		if err == nil {
			t.Fatal("unsafe plan was exported")
		}
		if _, err := os.Stat(directory); !os.IsNotExist(err) {
			t.Fatal("failed export left a partial package")
		}
	}
}

func TestGitDiffCanBeAppliedAndPreservesMissingFinalNewlines(t *testing.T) {
	// Arrange.
	existing := map[string]string{"file.txt": "before"}
	plan := domain.BootstrapPlan{Files: []domain.BootstrapFile{
		{Path: "file.txt", Status: "update", Content: "after"},
		{Path: "nested/new.txt", Status: "create", Content: "new\n"},
	}}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "file.txt"), []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act.
	diff, err := Diff(context.Background(), plan, existing)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "apply", "--check", "-")
	command.Dir = directory
	command.Stdin = strings.NewReader(string(diff))
	output, err := command.CombinedOutput()

	// Assert.
	if err != nil || !strings.Contains(string(diff), "No newline at end of file") {
		t.Fatalf("diff cannot be applied: %v %s\n%s", err, output, diff)
	}
}
