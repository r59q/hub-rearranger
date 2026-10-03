// Package agents consumes the public Agents contract without user credentials.
package agents

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
	contract "github.com/r59q/hub-rearranger/services/identity/internal/infrastructure/agents/contract"
)

type Planner struct {
	URL    string
	Client *http.Client
}

func (p Planner) Proposal(ctx context.Context, repo domain.Repository, review domain.BootstrapReview) (domain.BootstrapProposal, error) {
	method, endpoint := "GET", "/bootstrap"
	var payload io.Reader
	if review.Draft != nil {
		method, endpoint = "POST", "/profile-editor"
		encoded, err := json.Marshal(review.Draft)
		if err != nil || len(encoded) > 4096 {
			return domain.BootstrapProposal{}, domain.ErrInvalid
		}
		payload = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(p.URL, "/")+"/v1/repositories/"+url.PathEscape(repo.Owner)+"/"+url.PathEscape(repo.Name)+endpoint, payload)
	if err != nil {
		return domain.BootstrapProposal{}, domain.ErrUnavailable
	}
	request.Header.Set("Accept", "application/json")
	if review.Draft != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := p.Client.Do(request)
	if err != nil {
		return domain.BootstrapProposal{}, domain.ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return domain.BootstrapProposal{}, domain.ErrUnavailable
	}
	var preview contract.BootstrapPreview
	data, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil || len(data) > 8<<20 {
		return domain.BootstrapProposal{}, domain.ErrUnavailable
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&preview) != nil || decoder.Decode(new(any)) != io.EOF || preview.Repository != repo.Owner+"/"+repo.Name || len(preview.Files) == 0 || len(preview.Files) > 256 {
		return domain.BootstrapProposal{}, domain.ErrUnavailable
	}
	if string(preview.State) != "ready" || len(preview.Diagnostics) != 0 || preview.BaseRevision != review.BaseRevision || preview.Digest != review.Digest || preview.DefaultBranch == "" {
		return domain.BootstrapProposal{}, domain.ErrBootstrapStale
	}
	proposal := domain.BootstrapProposal{Repository: preview.Repository, DefaultBranch: preview.DefaultBranch, BaseRevision: preview.BaseRevision, Digest: preview.Digest, ProfileEdit: review.Draft != nil}
	paths := map[string]bool{}
	for _, file := range preview.Files {
		digest := sha256.Sum256([]byte(file.Content))
		if paths[file.Path] || file.Sha256 == nil || *file.Sha256 != hex.EncodeToString(digest[:]) || len(file.Content) > 1<<20 {
			return domain.BootstrapProposal{}, domain.ErrUnavailable
		}
		paths[file.Path] = true
		switch string(file.Status) {
		case "unchanged":
			continue
		case "create", "update":
			base := ""
			if file.BaseSha != nil {
				base = *file.BaseSha
			}
			if (string(file.Status) == "create" && base != "") || (string(file.Status) == "update" && base == "") {
				return domain.BootstrapProposal{}, domain.ErrUnavailable
			}
			proposal.Changes = append(proposal.Changes, domain.BootstrapChange{Path: file.Path, BaseSHA: base, Content: file.Content})
		default:
			return domain.BootstrapProposal{}, domain.ErrUnavailable
		}
	}
	return proposal, nil
}
