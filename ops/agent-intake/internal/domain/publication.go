package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const StaleHead Failure = "STALE_HEAD"
const ProtectedChange Failure = "PROTECTED_CHANGE"

// Proposal contains only origin-verified immutable artifact data. Validation is
// independently performed by the canonical schema adapter before publication.
type Proposal struct {
	Invocation     Invocation
	ArtifactID     int64
	Attempt        int
	Created        time.Time
	Result         json.RawMessage
	Patch, Summary []byte
}

type FileChange struct {
	Path    string `json:"path"`
	Mode    string `json:"mode"`
	Content []byte `json:"content"`
	Delete  bool   `json:"delete"`
}

type Publication struct {
	Branch   string `json:"branch"`
	HeadSHA  string `json:"head_sha"`
	PRNumber int    `json:"pr_number"`
	PRURL    string `json:"pr_url"`
}

type PublicationPort interface {
	Proposal(context.Context, Invocation) (Proposal, error)
	ExecutionSource(context.Context, Invocation) ([]byte, error)
	CommitProposal(context.Context, Invocation, Proposal, []FileChange) (string, error)
	EnsureBranch(context.Context, Invocation, string) error
	EnsureDraft(context.Context, Invocation, Proposal, string) (Publication, error)
	EnsurePublicationComment(context.Context, Invocation, Proposal, Publication) error
	EnsurePublicationCheck(context.Context, Invocation, Proposal, Publication) error
	ConfirmPublication(context.Context, Invocation, Proposal, Publication) error
}

type ProposalPolicyPort interface {
	VerifyProposal(context.Context, Invocation, Proposal) error
}

type PatchPort interface {
	Apply(context.Context, []byte, []byte) ([]FileChange, error)
}

type Publisher struct {
	authorization *Service
	repository    RepositoryPort
	publication   PublicationPort
	evidence      ProposalPolicyPort
	patch         PatchPort
}

func NewPublisher(authorization *Service, repository RepositoryPort, publication PublicationPort, evidence ProposalPolicyPort, patch PatchPort) *Publisher {
	return &Publisher{authorization, repository, publication, evidence, patch}
}

func PublicationBranch(input Invocation) string {
	return fmt.Sprintf("agent/codex-thorough/%d-%d", input.Repository.ID, input.RequestCommentID)
}

// guard is a live authorization boundary for each publication phase. An earlier
// intake/result/receipt never grants a later write. V1 does not rebase stale bases.
func (p *Publisher) guard(ctx context.Context, event Event, input Invocation) error {
	head, err := p.repository.Head(ctx, input.Repository)
	if err != nil {
		return err
	}
	if head != input.BaseSHA {
		return StaleHead
	}
	verified, err := p.authorization.Reauthorize(ctx, event, input)
	if err != nil {
		return err
	}
	if verified.PolicyRevision != input.PolicyRevision {
		return StaleHead
	}
	head, err = p.repository.Head(ctx, input.Repository)
	if err != nil {
		return err
	}
	if head != input.BaseSHA {
		return StaleHead
	}
	return nil
}

func (p *Publisher) prepare(ctx context.Context, event Event, input Invocation) (Proposal, string, error) {
	if err := p.guard(ctx, event, input); err != nil {
		return Proposal{}, "", err
	}
	proposal, err := p.publication.Proposal(ctx, input)
	if err != nil {
		return Proposal{}, "", err
	}
	if err = p.evidence.VerifyProposal(ctx, input, proposal); err != nil {
		return Proposal{}, "", err
	}
	archive, err := p.publication.ExecutionSource(ctx, input)
	if err != nil {
		return Proposal{}, "", err
	}
	if err = p.guard(ctx, event, input); err != nil {
		return Proposal{}, "", err
	}
	changes, err := p.patch.Apply(ctx, archive, proposal.Patch)
	if err != nil {
		return Proposal{}, "", err
	}
	if len(changes) == 0 || len(changes) > 1000 {
		return Proposal{}, "", ProtectedChange
	}
	if err = p.guard(ctx, event, input); err != nil {
		return Proposal{}, "", err
	}
	head, err := p.publication.CommitProposal(ctx, input, proposal, changes)
	if err != nil {
		return Proposal{}, "", err
	}
	if !ValidSHA(head) || head == input.BaseSHA {
		return Proposal{}, "", HistoryUnavailable
	}
	return proposal, head, nil
}

func (p *Publisher) Publish(ctx context.Context, event Event, input Invocation) (Publication, error) {
	proposal, head, err := p.prepare(ctx, event, input)
	if err != nil {
		return Publication{}, err
	}
	if err = p.guard(ctx, event, input); err != nil {
		return Publication{}, err
	}
	if err = p.publication.EnsureBranch(ctx, input, head); err != nil {
		return Publication{}, err
	}
	if err = p.guard(ctx, event, input); err != nil {
		return Publication{}, err
	}
	publication, err := p.publication.EnsureDraft(ctx, input, proposal, head)
	if err != nil {
		return Publication{}, err
	}
	if publication.HeadSHA != head || publication.Branch != PublicationBranch(input) || publication.PRNumber <= 0 || publication.PRURL != fmt.Sprintf("%s/pull/%d", RepositoryURL(input.Repository), publication.PRNumber) {
		return Publication{}, HistoryUnavailable
	}
	if err = p.guard(ctx, event, input); err != nil {
		return Publication{}, err
	}
	if err = p.publication.EnsurePublicationComment(ctx, input, proposal, publication); err != nil {
		return Publication{}, err
	}
	if err = p.guard(ctx, event, input); err != nil {
		return Publication{}, err
	}
	if err = p.publication.EnsurePublicationCheck(ctx, input, proposal, publication); err != nil {
		return Publication{}, err
	}
	if err = p.guard(ctx, event, input); err != nil {
		return Publication{}, err
	}
	if err = p.publication.ConfirmPublication(ctx, input, proposal, publication); err != nil {
		return Publication{}, err
	}
	return publication, nil
}
