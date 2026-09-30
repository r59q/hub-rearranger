package domain

import "context"

type Convention struct {
	Version              int
	ProfileCatalogPath   string
	ProfileSchemaVersion int
	Assignment           AssignmentPolicy
}

type AssignmentPolicy struct {
	Event           string
	SourceKind      string
	CommandTemplate string
	Authority       string
	RequiredRoles   []string
	ProfileRevision ProfileRevisionPolicy
}

type ProfileRevisionPolicy struct {
	Format                string
	DefaultBranchAncestor bool
}

// Service owns repository agent semantics, not execution or durable run state.
// Profile reads and readiness use cases will be added in AW-005/AW-006.
type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) AssignmentConvention(ctx context.Context) (Convention, error) {
	if err := ctx.Err(); err != nil {
		return Convention{}, err
	}
	return Convention{
		Version: 1, ProfileCatalogPath: ".github/agent-profiles.yml", ProfileSchemaVersion: 1,
		Assignment: AssignmentPolicy{
			Event: "issue_comment.created", SourceKind: "issue",
			CommandTemplate: "/agent assign {profile_id}@{profile_revision} authority=branch-draft-pr",
			Authority:       "branch-draft-pr", RequiredRoles: []string{"maintain", "admin"},
			ProfileRevision: ProfileRevisionPolicy{
				Format: "full-40-character-sha", DefaultBranchAncestor: true,
			},
		},
	}, nil
}
