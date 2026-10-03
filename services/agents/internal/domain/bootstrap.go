package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
)

const BootstrapMaxBytes = 1 << 20

// BootstrapFile is a proposed regular UTF-8 repository file, never a write grant.
type BootstrapFile struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	SHA256  string `json:"sha256,omitempty"`
	Content string `json:"content,omitempty"`
}

type BootstrapPlan struct {
	Version     int             `json:"version"`
	Files       []BootstrapFile `json:"files"`
	Diagnostics []Diagnostic    `json:"diagnostics"`
}

type BootstrapCatalogMerger interface {
	MergeBootstrapCatalog(existing, proposed []byte) ([]byte, []Diagnostic)
}

// PlanBootstrap is pure: callers provide canonical templates and a bounded
// target snapshot. GitHub reads/writes and runner provisioning belong elsewhere.
func PlanBootstrap(templates, existing map[string]string, merger BootstrapCatalogMerger) BootstrapPlan {
	plan := BootstrapPlan{Version: 1, Files: []BootstrapFile{}, Diagnostics: []Diagnostic{}}
	paths := make([]string, 0, len(templates))
	for path := range templates {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		content := templates[path]
		previous, exists := existing[path]
		var diagnostics []Diagnostic
		switch path {
		case ProfileCatalogPath:
			var merged []byte
			merged, diagnostics = merger.MergeBootstrapCatalog([]byte(previous), []byte(content))
			content = string(merged)
		case "AGENTS.md":
			content, diagnostics = mergeBootstrapInstructions(previous, content)
		}
		limit := BootstrapMaxBytes
		if slices.Contains(ReadinessFiles, path) {
			limit = ReadinessFileMaxBytes
		}
		if len(content) > limit {
			diagnostics = append(diagnostics, Diagnostic{Code: "LIMIT_EXCEEDED", Message: fmt.Sprintf("Reduce the proposed file to at most %d bytes before generating again.", limit)})
		}
		if len(diagnostics) > 0 {
			for _, diagnostic := range diagnostics {
				diagnostic.Path = path
				plan.Diagnostics = append(plan.Diagnostics, diagnostic)
			}
			plan.Files = append(plan.Files, BootstrapFile{Path: path, Status: "conflict"})
			continue
		}

		status := "create"
		if exists {
			status = "update"
			if previous == content {
				status = "unchanged"
			}
		}
		digest := sha256.Sum256([]byte(content))
		plan.Files = append(plan.Files, BootstrapFile{Path: path, Status: status,
			SHA256: hex.EncodeToString(digest[:]), Content: content})
	}
	return plan
}
