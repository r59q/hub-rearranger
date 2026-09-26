package github

import (
	"context"
	"fmt"

	gh "github.com/google/go-github/v74/github"

	"github.com/r59q/hub-rearranger/services/repositories/internal/domain"
)

type repositoriesClient interface {
	List(context.Context, string, *gh.RepositoryListOptions) ([]*gh.Repository, *gh.Response, error)
}

type Catalog struct {
	repositories repositoriesClient
}

func NewCatalog(client *gh.Client) *Catalog {
	return &Catalog{repositories: client.Repositories}
}

func (c *Catalog) ListForAccount(ctx context.Context) ([]domain.Repository, error) {
	options := &gh.RepositoryListOptions{
		Affiliation: "owner,collaborator,organization_member",
		Sort:        "full_name",
		Direction:   "asc",
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	repositories := make([]domain.Repository, 0)

	for {
		page, response, err := c.repositories.List(ctx, "", options)
		if err != nil {
			return nil, fmt.Errorf("GitHub list repositories: %w: %v", domain.ErrCatalogUnavailable, err)
		}
		for _, repository := range page {
			if repository == nil || repository.GetID() <= 0 {
				continue
			}
			repositories = append(repositories, domain.Repository{
				ID:            repository.GetID(),
				Owner:         repository.GetOwner().GetLogin(),
				Name:          repository.GetName(),
				FullName:      repository.GetFullName(),
				HTMLURL:       repository.GetHTMLURL(),
				Description:   repository.GetDescription(),
				Private:       repository.GetPrivate(),
				DefaultBranch: repository.GetDefaultBranch(),
			})
		}

		if response == nil || response.NextPage == 0 {
			break
		}
		options.Page = response.NextPage
	}

	return repositories, nil
}
