package domain

import (
	"context"
	"errors"
	"testing"
)

type catalogReader struct {
	file  CatalogFile
	err   error
	calls int
}

func (r *catalogReader) ReadCatalog(ctx context.Context, _ Repository) (CatalogFile, error) {
	r.calls++
	if err := ctx.Err(); err != nil {
		return CatalogFile{}, err
	}
	return r.file, r.err
}

type catalogValidator struct {
	profiles    map[string]map[string]any
	diagnostics []Diagnostic
	calls       int
}

func (v *catalogValidator) Validate(_ []byte) (map[string]map[string]any, []Diagnostic) {
	v.calls++
	return v.profiles, v.diagnostics
}

func TestProfileReadsFetchFreshPinnedDataAndSortProfiles(t *testing.T) {
	// Arrange.
	reader := &catalogReader{file: CatalogFile{Revision: "first", DefaultBranch: "main"}}
	validator := &catalogValidator{profiles: map[string]map[string]any{"z-last": {"enabled": false}, "a-first": {"enabled": true}}, diagnostics: []Diagnostic{}}
	service := NewProfileService(reader, validator)
	// Act.
	first, err := service.RepositoryProfiles(context.Background(), Repository{Owner: "octo", Name: "demo"})
	reader.file.Revision = "second"
	second, secondErr := service.RepositoryProfiles(context.Background(), Repository{Owner: "octo", Name: "demo"})
	// Assert.
	if err != nil || secondErr != nil || first.State != "valid" || first.Repository != "octo/demo" || first.Profiles[0].ID != "a-first" {
		t.Fatalf("result = %v, error = %v", first, err)
	}
	if reader.calls != 2 || first.Profiles[0].Revision != "first" || second.Profiles[0].Revision != "second" {
		t.Fatal("profile revision was cached or not pinned")
	}
}

func TestProfileValidationStatesNeverExposePartialProfiles(t *testing.T) {
	for code, state := range map[string]string{"MISSING_FILE": "missing", "UNSUPPORTED_VERSION": "unsupported", "INVALID_YAML": "invalid", "UNSUPPORTED_VALUE": "invalid"} {
		t.Run(code, func(t *testing.T) {
			// Arrange.
			validator := &catalogValidator{profiles: map[string]map[string]any{"partial": {}}, diagnostics: []Diagnostic{{Code: code, Path: "/", Message: "Fix the catalog."}}}
			reader := &catalogReader{}
			if code == "MISSING_FILE" {
				reader.file.Problem = &validator.diagnostics[0]
			}
			service := NewProfileService(reader, validator)
			// Act.
			result, err := service.RepositoryProfiles(context.Background(), Repository{})
			// Assert.
			if err != nil || result.State != state || len(result.Profiles) != 0 || len(result.Diagnostics) != 1 {
				t.Fatalf("result = %v, error = %v", result, err)
			}
			if code == "MISSING_FILE" && validator.calls != 0 {
				t.Fatal("missing file was parsed")
			}
		})
	}
}

func TestProfileReadsHonorCancellationAndUpstreamFailures(t *testing.T) {
	// Arrange.
	reader := &catalogReader{err: ErrAccessDenied}
	validator := &catalogValidator{}
	service := NewProfileService(reader, validator)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Act.
	_, canceled := service.RepositoryProfiles(ctx, Repository{})
	_, denied := service.RepositoryProfiles(context.Background(), Repository{})
	// Assert.
	if !errors.Is(canceled, context.Canceled) || !errors.Is(denied, ErrAccessDenied) || reader.calls != 1 || validator.calls != 0 {
		t.Fatalf("canceled = %v, denied = %v", canceled, denied)
	}
}
