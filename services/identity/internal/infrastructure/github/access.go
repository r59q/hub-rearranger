package github

import (
	"context"
	"strings"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

func (p *Provider) RepositoryAccess(ctx context.Context, token string, user domain.User, repo domain.Repository) (domain.Access, error) {
	client := p.api.WithAuthToken(token)
	repository, response, err := client.Repositories.Get(ctx, repo.Owner, repo.Name)
	if err != nil {
		return domain.Access{}, apiError(response, false)
	}
	if repository.GetID() <= 0 || !strings.EqualFold(repository.GetFullName(), repo.Owner+"/"+repo.Name) || repository.GetArchived() || repository.GetDisabled() {
		return domain.Access{}, domain.ErrForbidden
	}

	permission, response, err := client.Repositories.GetPermissionLevel(ctx, repo.Owner, repo.Name, user.Login)
	if err != nil {
		return domain.Access{}, apiError(response, false)
	}
	if permission.GetUser().GetID() != user.ID {
		return domain.Access{}, domain.ErrForbidden
	}

	permissions, err := p.installationPermissions(ctx, client, repository.GetID())
	if err != nil {
		return domain.Access{}, err
	}

	// GitHub's coarse permission "write" includes maintain. Use the exact role,
	// matching the GitHub-native intake's maintain/admin policy.
	return domain.Access{RepositoryID: repository.GetID(), FullName: repository.GetFullName(), Role: permission.GetRoleName(),
		IssuesWrite: permissions.GetIssues() == "write", ContentsWrite: permissions.GetContents() == "write",
		PullRequestsWrite: permissions.GetPullRequests() == "write", WorkflowsWrite: permissions.GetWorkflows() == "write"}, nil
}

func (p *Provider) installationPermissions(ctx context.Context, client *gh.Client, repositoryID int64) (*gh.InstallationPermissions, error) {
	// Read current installed permissions and repository selection, including on public
	// repositories. A successful public GET alone does not prove App write access.
	options := &gh.ListOptions{PerPage: 100, Page: 1}
	for pages := 0; pages < 20; pages++ {
		installations, response, err := client.Apps.ListUserInstallations(ctx, options)
		if err != nil {
			return nil, apiError(response, false)
		}

		for _, installation := range installations {
			if installation.GetAppID() != p.appID || installation.GetID() <= 0 || installation.SuspendedAt != nil {
				continue
			}

			found, err := installationHasRepository(ctx, client, installation.GetID(), repositoryID)
			if err != nil {
				return nil, err
			}
			if found {
				return installation.Permissions, nil
			}
		}

		if response.NextPage == 0 {
			return nil, domain.ErrForbidden
		}
		options.Page = response.NextPage
	}
	return nil, domain.ErrUnavailable
}

func installationHasRepository(ctx context.Context, client *gh.Client, installationID, repositoryID int64) (bool, error) {
	options := &gh.ListOptions{PerPage: 100, Page: 1}
	for pages := 0; pages < 100; pages++ {
		repositories, response, err := client.Apps.ListUserRepos(ctx, installationID, options)
		if err != nil {
			return false, apiError(response, false)
		}

		for _, repository := range repositories.Repositories {
			if repository.GetID() == repositoryID {
				return true, nil
			}
		}

		if response.NextPage == 0 {
			return false, nil
		}
		options.Page = response.NextPage
	}
	return false, domain.ErrUnavailable
}
