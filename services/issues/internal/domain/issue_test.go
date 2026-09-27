package domain

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"
)

type fakeCatalog struct {
	issues      map[string][]Issue
	listErrors  map[string]error
	details     map[int]IssueDetails
	detailCalls []int
	detailError error
	mutex       sync.Mutex
}

func (f *fakeCatalog) ListByRepository(_ context.Context, repository Repository, _ int) ([]Issue, error) {
	return append([]Issue(nil), f.issues[repository.FullName()]...), f.listErrors[repository.FullName()]
}

func (f *fakeCatalog) GetDetails(_ context.Context, _ Repository, number int) (IssueDetails, error) {
	f.mutex.Lock()
	f.detailCalls = append(f.detailCalls, number)
	f.mutex.Unlock()
	return f.details[number], f.detailError
}

func TestListRecentSortsPaginatesAndEnrichesVisibleIssues(t *testing.T) {
	// Arrange.
	now := time.Now()
	catalog := &fakeCatalog{
		issues: map[string][]Issue{
			"octo/alpha": {{Number: 1, Repository: "octo/alpha", UpdatedAt: now.Add(-time.Hour)}},
			"octo/beta": {
				{Number: 3, Repository: "octo/beta", UpdatedAt: now},
				{Number: 2, Repository: "octo/beta", UpdatedAt: now.Add(-2 * time.Hour)},
			},
		},
		listErrors: map[string]error{},
		details: map[int]IssueDetails{
			1: {Subtasks: []RelatedIssue{{Number: 9}}},
			3: {LinkedPullRequests: []RelatedIssue{{Number: 4}}},
		},
	}
	service := NewIssueService(catalog)

	// Act.
	page, err := service.ListRecent(context.Background(), []Repository{{Owner: "octo", Name: "alpha"}, {Owner: "octo", Name: "beta"}}, 1, 2)

	// Assert.
	if err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}
	if got := []int{page.Issues[0].Number, page.Issues[1].Number}; !reflect.DeepEqual(got, []int{3, 1}) {
		t.Fatalf("issue order = %v", got)
	}
	sort.Ints(catalog.detailCalls)
	if !page.HasNext || !reflect.DeepEqual(catalog.detailCalls, []int{1, 3}) {
		t.Fatalf("has next = %v, detail calls = %v", page.HasNext, catalog.detailCalls)
	}
	if len(page.Issues[0].LinkedPullRequests) != 1 || len(page.Issues[1].Subtasks) != 1 {
		t.Fatalf("enriched issues = %#v", page.Issues)
	}
}

func TestListRecentReturnsPartialRepositoryResults(t *testing.T) {
	// Arrange.
	catalog := &fakeCatalog{
		issues:     map[string][]Issue{"octo/alpha": {{Number: 1, Repository: "octo/alpha"}}},
		listErrors: map[string]error{"octo/beta": errors.New("forbidden")},
		details:    map[int]IssueDetails{},
	}
	service := NewIssueService(catalog)

	// Act.
	page, err := service.ListRecent(context.Background(), []Repository{{Owner: "octo", Name: "alpha"}, {Owner: "octo", Name: "beta"}}, 1, 6)

	// Assert.
	if err != nil || !reflect.DeepEqual(page.UnavailableRepositories, []string{"octo/beta"}) || len(page.Issues) != 1 {
		t.Fatalf("page = %#v, error = %v", page, err)
	}
}

func TestListRecentFailsWhenEveryRepositoryIsUnavailable(t *testing.T) {
	// Arrange.
	service := NewIssueService(&fakeCatalog{
		issues:     map[string][]Issue{},
		listErrors: map[string]error{"octo/alpha": errors.New("rate limited")},
	})

	// Act.
	_, err := service.ListRecent(context.Background(), []Repository{{Owner: "octo", Name: "alpha"}}, 1, 6)

	// Assert.
	if !errors.Is(err, ErrIssuesUnavailable) {
		t.Fatalf("error = %v", err)
	}
}
