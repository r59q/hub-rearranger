package domain

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestConventionRestrictsAssignmentAuthorityAndPinsRepositoryCatalog(t *testing.T) {
	// Arrange.
	service := NewService()

	// Act.
	convention, err := service.AssignmentConvention(context.Background())

	// Assert.
	if err != nil {
		t.Fatal(err)
	}
	if convention.Version != 1 || convention.ProfileSchemaVersion != 1 || convention.ProfileCatalogPath != ".github/agent-profiles.yml" {
		t.Fatalf("catalog convention = %#v", convention)
	}
	policy := convention.Assignment
	if policy.Authority != "branch-draft-pr" || policy.SourceKind != "issue" || policy.Event != "issue_comment.created" {
		t.Fatalf("assignment policy = %#v", policy)
	}
	if policy.CommandTemplate != "/agent assign {profile_id}@{profile_revision} authority=branch-draft-pr" ||
		!reflect.DeepEqual(policy.RequiredRoles, []string{"maintain", "admin"}) ||
		policy.ProfileRevision.Format != "full-40-character-sha" || !policy.ProfileRevision.DefaultBranchAncestor {
		t.Fatalf("assignment authorization = %#v", policy)
	}
}

func TestConventionHonorsCancellation(t *testing.T) {
	// Arrange.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := NewService()

	// Act.
	_, err := service.AssignmentConvention(ctx)

	// Assert.
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestConventionHasNoMutableStateBetweenRequests(t *testing.T) {
	// Arrange.
	service := NewService()
	previous, err := service.AssignmentConvention(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	previous.Assignment.RequiredRoles[0] = "write"

	// Act.
	current, err := service.AssignmentConvention(context.Background())

	// Assert.
	if err != nil || !reflect.DeepEqual(current.Assignment.RequiredRoles, []string{"maintain", "admin"}) {
		t.Fatalf("roles = %v, error = %v", current.Assignment.RequiredRoles, err)
	}
}
