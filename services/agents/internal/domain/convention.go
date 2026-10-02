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
// Profile validation is independent of runtime readiness.
type Service struct {
	profiles  *ProfileService
	readiness *ReadinessService
}

func NewService(profiles *ProfileService, readiness ...*ReadinessService) *Service {
	s := &Service{profiles: profiles}
	if len(readiness) > 0 {
		s.readiness = readiness[0]
	}
	return s
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

func (s *Service) RepositoryProfiles(ctx context.Context, repository Repository) (ProfileCatalog, error) {
	if s.profiles == nil {
		return ProfileCatalog{}, ErrGitHubUnavailable
	}
	return s.profiles.RepositoryProfiles(ctx, repository)
}

func (s *Service) RepositoryReadiness(ctx context.Context, repository Repository) (RepositoryReadiness, error) {
	if s.readiness == nil {
		return RepositoryReadiness{}, ErrGitHubUnavailable
	}
	return s.readiness.RepositoryReadiness(ctx, repository)
}
