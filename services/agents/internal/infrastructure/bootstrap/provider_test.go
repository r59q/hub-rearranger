package bootstrap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/profiles"
)

func TestContainerBundleMatchesCanonicalGeneratorAndRejectsTampering(t *testing.T) {
	for _, variant := range []string{"canonical", "hash", "path", "missing", "version"} {
		t.Run(variant, func(t *testing.T) {
			templates := canonicalTemplates(t)
			validator, err := profiles.NewValidator()
			if err != nil {
				t.Fatal(err)
			}
			plan := domain.PlanBootstrap(templates, nil, validator)
			switch variant {
			case "hash":
				plan.Files[0].Content = "tampered"
			case "path":
				plan.Files[0].Path = "../escape"
			case "missing":
				plan.Files = plan.Files[:1]
			case "version":
				plan.Version = 2
			}
			data, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "bootstrap.json")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			actual, err := (Provider{Bundle: path}).ReadTemplates(context.Background())
			if variant == "canonical" {
				if err != nil || len(actual) != len(templates) {
					t.Fatal("container package diverged", err)
				}
				for path, content := range templates {
					if actual[path] != content {
						t.Fatal("source bytes changed")
					}
				}
			} else if err == nil {
				t.Fatal("bad bundle accepted")
			}
		})
	}
}
