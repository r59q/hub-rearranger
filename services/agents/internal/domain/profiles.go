package domain

import (
	"context"
	"errors"
	"sort"
)

const ProfileCatalogPath = ".github/agent-profiles.yml"

var (
	ErrRepositoryUnavailable = errors.New("repository unavailable")
	ErrGitHubUnavailable     = errors.New("github unavailable")
	ErrAccessDenied          = errors.New("repository access denied")
	ErrRateLimited           = errors.New("github rate limited")
)

type Repository struct{ Owner, Name string }

func (r Repository) FullName() string { return r.Owner + "/" + r.Name }

type Diagnostic struct{ Code, Path, Message string }

// CatalogFile is an ephemeral GitHub projection pinned to a commit, never a
// blob SHA or a mutable branch name. Problem describes a missing/unsupported file.
type CatalogFile struct {
	DefaultBranch, Revision string
	Content                 []byte
	Problem                 *Diagnostic
}

type ProfileReader interface {
	ReadCatalog(context.Context, Repository) (CatalogFile, error)
}
type ProfileValidator interface {
	Validate([]byte) (map[string]map[string]any, []Diagnostic)
}

// Configuration is validated JSON-shaped data, independent of transport DTOs.
// Invalid catalogs never expose partially parsed configuration or source text.
type Profile struct {
	ID, Revision  string
	Configuration map[string]any
}

type ProfileCatalog struct {
	Repository, DefaultBranch, Revision, State string
	Profiles                                   []Profile
	Diagnostics                                []Diagnostic
}

type ProfileService struct {
	reader    ProfileReader
	validator ProfileValidator
}

func NewProfileService(reader ProfileReader, validator ProfileValidator) *ProfileService {
	return &ProfileService{reader: reader, validator: validator}
}

func (s *ProfileService) RepositoryProfiles(ctx context.Context, repository Repository) (ProfileCatalog, error) {
	if err := ctx.Err(); err != nil {
		return ProfileCatalog{}, err
	}
	file, err := s.reader.ReadCatalog(ctx, repository)
	if err != nil {
		return ProfileCatalog{}, err
	}
	if err := ctx.Err(); err != nil {
		return ProfileCatalog{}, err
	}
	result := ProfileCatalog{Repository: repository.FullName(), DefaultBranch: file.DefaultBranch,
		Revision: file.Revision, State: "valid", Profiles: []Profile{}, Diagnostics: []Diagnostic{}}
	if file.Problem != nil {
		result.Diagnostics = append(result.Diagnostics, *file.Problem)
	} else {
		profiles, diagnostics := s.validator.Validate(file.Content)
		result.Diagnostics = diagnostics
		if len(diagnostics) == 0 {
			for id, configuration := range profiles {
				result.Profiles = append(result.Profiles, Profile{ID: id, Revision: file.Revision, Configuration: configuration})
			}
			sort.Slice(result.Profiles, func(i, j int) bool { return result.Profiles[i].ID < result.Profiles[j].ID })
		}
	}
	if len(result.Diagnostics) > 0 {
		result.State = "invalid"
		switch result.Diagnostics[0].Code {
		case "MISSING_FILE":
			result.State = "missing"
		case "UNSUPPORTED_VERSION":
			result.State = "unsupported"
		}
	}
	return result, nil
}
