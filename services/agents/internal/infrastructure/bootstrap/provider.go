package bootstrap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

type Provider struct{ Source, Bundle string }

func (p Provider) ReadTemplates(ctx context.Context) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.Bundle == "" {
		root, err := os.OpenRoot(p.Source)
		if err != nil {
			return nil, ErrTemplates
		}
		defer root.Close()
		return Templates(root)
	}
	file, err := os.Open(p.Bundle)
	if err != nil {
		return nil, ErrTemplates
	}
	defer file.Close()
	var plan domain.BootstrapPlan
	data, err := io.ReadAll(io.LimitReader(file, (8<<20)+1))
	if err != nil || len(data) > 8<<20 {
		return nil, ErrTemplates
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&plan) != nil || decoder.Decode(new(any)) != io.EOF || plan.Version != 1 || len(plan.Diagnostics) != 0 || len(plan.Files) == 0 || len(plan.Files) > 256 {
		return nil, ErrTemplates
	}
	templates := map[string]string{}
	for _, file := range plan.Files {
		digest := sha256.Sum256([]byte(file.Content))
		if !fs.ValidPath(file.Path) || file.Path == "." || strings.Contains(file.Path, "\\") || file.Status != "create" || file.Content == "" || !utf8.ValidString(file.Content) || strings.ContainsRune(file.Content, 0) || file.SHA256 != hex.EncodeToString(digest[:]) || len(file.Content) > domain.BootstrapMaxBytes || templates[file.Path] != "" {
			return nil, ErrTemplates
		}
		templates[file.Path] = file.Content
	}
	for _, path := range append([]string{"AGENTS.md"}, packageFiles...) {
		if templates[path] == "" {
			return nil, ErrTemplates
		}
	}
	return templates, nil
}

func (p Provider) Render(ctx context.Context, plan domain.BootstrapPlan, existing map[string]string) (string, error) {
	content, err := Diff(ctx, plan, existing)
	return string(content), err
}
