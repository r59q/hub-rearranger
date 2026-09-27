package domain

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var ErrIssuesUnavailable = errors.New("issue catalog unavailable")

type Repository struct {
	Owner string
	Name  string
}

func (r Repository) FullName() string {
	return r.Owner + "/" + r.Name
}

type Label struct {
	Name  string
	Color string
}

type RelatedIssue struct {
	Number     int
	Repository string
	Title      string
	State      string
	HTMLURL    string
}

type IssueDetails struct {
	Subtasks           []RelatedIssue
	LinkedIssues       []RelatedIssue
	LinkedPullRequests []RelatedIssue
}

type Issue struct {
	ID                 int64
	Number             int
	Repository         string
	Title              string
	Description        string
	State              string
	HTMLURL            string
	UpdatedAt          time.Time
	Labels             []Label
	Subtasks           []RelatedIssue
	LinkedIssues       []RelatedIssue
	LinkedPullRequests []RelatedIssue
	DetailsAvailable   bool
}

type Page struct {
	Issues                  []Issue
	Page                    int
	PerPage                 int
	HasNext                 bool
	UnavailableRepositories []string
	IncompleteDetails       int
}

type Catalog interface {
	ListByRepository(context.Context, Repository, int) ([]Issue, error)
	GetDetails(context.Context, Repository, int) (IssueDetails, error)
}

type IssueService struct {
	catalog Catalog
}

func NewIssueService(catalog Catalog) *IssueService {
	return &IssueService{catalog: catalog}
}

func (s *IssueService) ListRecent(ctx context.Context, repositories []Repository, page, perPage int) (Page, error) {
	if page < 1 || perPage < 1 {
		return Page{}, fmt.Errorf("page and per-page values must be positive")
	}

	end := page * perPage
	issues, unavailable := s.listRepositoryIssues(ctx, repositories, end+1)
	if len(repositories) > 0 && len(unavailable) == len(repositories) {
		return Page{}, fmt.Errorf("%w: all selected repositories failed", ErrIssuesUnavailable)
	}

	sortRecentIssues(issues)
	result := paginateIssues(issues, page, perPage, unavailable)
	if len(result.Issues) > 0 {
		s.enrichIssues(ctx, repositories, &result)
	}

	return result, nil
}

func (s *IssueService) listRepositoryIssues(ctx context.Context, repositories []Repository, limit int) ([]Issue, []string) {
	results := make([]repositoryIssues, len(repositories))
	var workers sync.WaitGroup
	semaphore := make(chan struct{}, 8)
	for i, repository := range repositories {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := acquire(ctx, semaphore); err != nil {
				results[i].err = err
				return
			}
			defer release(semaphore)
			results[i].issues, results[i].err = s.catalog.ListByRepository(ctx, repository, limit)
		}()
	}
	workers.Wait()

	issues := make([]Issue, 0)
	unavailable := make([]string, 0)
	for i, result := range results {
		if result.err != nil {
			unavailable = append(unavailable, repositories[i].FullName())
			continue
		}
		issues = append(issues, result.issues...)
	}
	return issues, unavailable
}

func sortRecentIssues(issues []Issue) {
	sort.SliceStable(issues, func(i, j int) bool {
		left, right := issues[i], issues[j]
		if left.UpdatedAt.Equal(right.UpdatedAt) {
			if left.Repository == right.Repository {
				return left.Number > right.Number
			}
			return left.Repository < right.Repository
		}
		return left.UpdatedAt.After(right.UpdatedAt)
	})
}

func paginateIssues(issues []Issue, page, perPage int, unavailable []string) Page {
	start := (page - 1) * perPage
	end := page * perPage
	result := Page{
		Issues:                  []Issue{},
		Page:                    page,
		PerPage:                 perPage,
		HasNext:                 len(issues) > end,
		UnavailableRepositories: unavailable,
	}
	if start >= len(issues) {
		return result
	}
	if end > len(issues) {
		end = len(issues)
	}
	result.Issues = append(result.Issues, issues[start:end]...)
	return result
}

func (s *IssueService) enrichIssues(ctx context.Context, repositories []Repository, page *Page) {
	details := make([]IssueDetails, len(page.Issues))
	detailErrors := make([]error, len(page.Issues))
	var workers sync.WaitGroup
	semaphore := make(chan struct{}, 8)
	for i, issue := range page.Issues {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := acquire(ctx, semaphore); err != nil {
				detailErrors[i] = err
				return
			}
			defer release(semaphore)
			repository := repositoryByName(repositories, issue.Repository)
			details[i], detailErrors[i] = s.catalog.GetDetails(ctx, repository, issue.Number)
		}()
	}
	workers.Wait()

	for i := range page.Issues {
		if detailErrors[i] != nil {
			page.IncompleteDetails++
			continue
		}
		page.Issues[i].setDetails(details[i])
	}
}

func (issue *Issue) setDetails(details IssueDetails) {
	issue.Subtasks = nonNilRelated(details.Subtasks)
	issue.LinkedIssues = nonNilRelated(details.LinkedIssues)
	issue.LinkedPullRequests = nonNilRelated(details.LinkedPullRequests)
	issue.DetailsAvailable = true
}

func acquire(ctx context.Context, semaphore chan struct{}) error {
	select {
	case semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func release(semaphore chan struct{}) {
	<-semaphore
}

type repositoryIssues struct {
	issues []Issue
	err    error
}

func repositoryByName(repositories []Repository, fullName string) Repository {
	for _, repository := range repositories {
		if repository.FullName() == fullName {
			return repository
		}
	}
	return Repository{}
}

func nonNilRelated(issues []RelatedIssue) []RelatedIssue {
	if issues == nil {
		return []RelatedIssue{}
	}
	return issues
}
