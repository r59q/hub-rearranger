package github

import (
	"context"
	"fmt"
	"strings"

	gh "github.com/google/go-github/v74/github"

	"github.com/r59q/hub-rearranger/services/issues/internal/domain"
)

type issuesClient interface {
	ListByRepo(context.Context, string, string, *gh.IssueListByRepoOptions) ([]*gh.Issue, *gh.Response, error)
	ListIssueTimeline(context.Context, string, string, int, *gh.ListOptions) ([]*gh.Timeline, *gh.Response, error)
}

type subIssuesClient interface {
	ListByIssue(context.Context, string, string, int64, *gh.IssueListOptions) ([]*gh.SubIssue, *gh.Response, error)
}

type Catalog struct {
	issues    issuesClient
	subIssues subIssuesClient
}

func NewCatalog(client *gh.Client) *Catalog {
	return &Catalog{issues: client.Issues, subIssues: client.SubIssue}
}

func (c *Catalog) ListByRepository(ctx context.Context, repository domain.Repository, limit int) ([]domain.Issue, error) {
	options := &gh.IssueListByRepoOptions{
		State: "open", Sort: "updated", Direction: "desc",
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	issues := make([]domain.Issue, 0, limit)
	for len(issues) < limit {
		page, response, err := c.issues.ListByRepo(ctx, repository.Owner, repository.Name, options)
		if err != nil {
			return nil, fmt.Errorf("GitHub list issues for %s: %w: %v", repository.FullName(), domain.ErrIssuesUnavailable, err)
		}
		for _, issue := range page {
			if issue == nil || issue.IsPullRequest() || issue.GetID() <= 0 || issue.GetNumber() <= 0 {
				continue
			}
			issues = append(issues, mapIssue(repository, issue))
			if len(issues) == limit {
				break
			}
		}
		if response == nil || response.NextPage == 0 {
			break
		}
		options.ListOptions.Page = response.NextPage
	}
	return issues, nil
}

func (c *Catalog) GetDetails(ctx context.Context, repository domain.Repository, number int) (domain.IssueDetails, error) {
	subtasks, err := c.listSubtasks(ctx, repository, number)
	if err != nil {
		return domain.IssueDetails{}, err
	}
	linkedIssues, linkedPullRequests, err := c.listLinks(ctx, repository, number)
	if err != nil {
		return domain.IssueDetails{}, err
	}
	return domain.IssueDetails{
		Subtasks: subtasks, LinkedIssues: linkedIssues, LinkedPullRequests: linkedPullRequests,
	}, nil
}

func (c *Catalog) listSubtasks(ctx context.Context, repository domain.Repository, number int) ([]domain.RelatedIssue, error) {
	options := &gh.IssueListOptions{ListOptions: gh.ListOptions{PerPage: 100}}
	result := make([]domain.RelatedIssue, 0)
	for {
		page, response, err := c.subIssues.ListByIssue(ctx, repository.Owner, repository.Name, int64(number), options)
		if err != nil {
			return nil, fmt.Errorf("GitHub list sub-issues for %s#%d: %w", repository.FullName(), number, err)
		}
		for _, issue := range page {
			if issue != nil {
				result = append(result, mapRelated((*gh.Issue)(issue), repository.FullName()))
			}
		}
		if response == nil || response.NextPage == 0 {
			break
		}
		options.ListOptions.Page = response.NextPage
	}
	return result, nil
}

func (c *Catalog) listLinks(ctx context.Context, repository domain.Repository, number int) ([]domain.RelatedIssue, []domain.RelatedIssue, error) {
	options := &gh.ListOptions{PerPage: 100}
	linkedIssues := make([]domain.RelatedIssue, 0)
	linkedPullRequests := make([]domain.RelatedIssue, 0)
	seen := make(map[string]bool)
	for {
		page, response, err := c.issues.ListIssueTimeline(ctx, repository.Owner, repository.Name, number, options)
		if err != nil {
			return nil, nil, fmt.Errorf("GitHub list issue timeline for %s#%d: %w", repository.FullName(), number, err)
		}
		for _, event := range page {
			if event == nil || event.GetEvent() != "cross-referenced" || event.Source == nil || event.Source.Issue == nil {
				continue
			}
			issue := event.Source.Issue
			key := issue.GetHTMLURL()
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			related := mapRelated(issue, repositoryName(issue.GetRepositoryURL(), repository.FullName()))
			if issue.IsPullRequest() {
				linkedPullRequests = append(linkedPullRequests, related)
			} else {
				linkedIssues = append(linkedIssues, related)
			}
		}
		if response == nil || response.NextPage == 0 {
			break
		}
		options.Page = response.NextPage
	}
	return linkedIssues, linkedPullRequests, nil
}

func mapIssue(repository domain.Repository, issue *gh.Issue) domain.Issue {
	labels := make([]domain.Label, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		if label != nil {
			labels = append(labels, domain.Label{Name: label.GetName(), Color: label.GetColor()})
		}
	}
	return domain.Issue{
		ID: issue.GetID(), Number: issue.GetNumber(), Repository: repository.FullName(),
		Title: issue.GetTitle(), Description: issue.GetBody(), State: issue.GetState(),
		HTMLURL: issue.GetHTMLURL(), UpdatedAt: issue.GetUpdatedAt().Time, Labels: labels,
		Subtasks: []domain.RelatedIssue{}, LinkedIssues: []domain.RelatedIssue{},
		LinkedPullRequests: []domain.RelatedIssue{},
	}
}

func mapRelated(issue *gh.Issue, fallbackRepository string) domain.RelatedIssue {
	repository := fallbackRepository
	if issue.Repository != nil && issue.Repository.GetFullName() != "" {
		repository = issue.Repository.GetFullName()
	} else if parsed := repositoryName(issue.GetRepositoryURL(), ""); parsed != "" {
		repository = parsed
	}
	return domain.RelatedIssue{
		Number: issue.GetNumber(), Repository: repository, Title: issue.GetTitle(),
		State: issue.GetState(), HTMLURL: issue.GetHTMLURL(),
	}
}

func repositoryName(repositoryURL, fallback string) string {
	const marker = "/repos/"
	index := strings.LastIndex(repositoryURL, marker)
	if index < 0 {
		return fallback
	}
	name := strings.Trim(repositoryURL[index+len(marker):], "/")
	if strings.Count(name, "/") != 1 {
		return fallback
	}
	return name
}
