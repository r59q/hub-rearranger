package domain

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

var ErrCatalogUnavailable = errors.New("repository catalog unavailable")

type Repository struct {
	ID            int64
	Owner         string
	Name          string
	FullName      string
	HTMLURL       string
	Description   string
	Private       bool
	DefaultBranch string
	Selected      bool
}

type Catalog interface {
	ListForAccount(context.Context) ([]Repository, error)
}

type SelectionStore interface {
	List(context.Context) ([]int64, error)
	Replace(context.Context, []int64) error
}

type UnknownRepositoriesError struct {
	IDs []int64
}

func (e *UnknownRepositoriesError) Error() string {
	return fmt.Sprintf("repository IDs are not available: %v", e.IDs)
}

type RepositoryService struct {
	catalog Catalog
	store   SelectionStore
}

func NewRepositoryService(catalog Catalog, store SelectionStore) *RepositoryService {
	return &RepositoryService{catalog: catalog, store: store}
}

func (s *RepositoryService) ListAvailable(ctx context.Context) ([]Repository, error) {
	repositories, err := s.catalog.ListForAccount(ctx)
	if err != nil {
		return nil, fmt.Errorf("list account repositories: %w", err)
	}

	selectedIDs, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list repository selection: %w", err)
	}

	selected := idSet(selectedIDs)
	for i := range repositories {
		repositories[i].Selected = selected[repositories[i].ID]
	}
	sortRepositories(repositories)

	return repositories, nil
}

func (s *RepositoryService) ListSelected(ctx context.Context) ([]Repository, error) {
	repositories, err := s.ListAvailable(ctx)
	if err != nil {
		return nil, err
	}

	selected := make([]Repository, 0)
	for _, repository := range repositories {
		if repository.Selected {
			selected = append(selected, repository)
		}
	}

	return selected, nil
}

func (s *RepositoryService) ReplaceSelection(ctx context.Context, repositoryIDs []int64) ([]Repository, error) {
	repositories, err := s.catalog.ListForAccount(ctx)
	if err != nil {
		return nil, fmt.Errorf("list account repositories: %w", err)
	}

	available := make(map[int64]Repository, len(repositories))
	for _, repository := range repositories {
		available[repository.ID] = repository
	}

	uniqueIDs := unique(repositoryIDs)
	unknown := make([]int64, 0)
	for _, id := range uniqueIDs {
		if id <= 0 {
			unknown = append(unknown, id)
			continue
		}
		if _, ok := available[id]; !ok {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) > 0 {
		sort.Slice(unknown, func(i, j int) bool { return unknown[i] < unknown[j] })
		return nil, &UnknownRepositoriesError{IDs: unknown}
	}

	if err := s.store.Replace(ctx, uniqueIDs); err != nil {
		return nil, fmt.Errorf("replace repository selection: %w", err)
	}

	selected := make([]Repository, 0, len(uniqueIDs))
	for _, id := range uniqueIDs {
		repository := available[id]
		repository.Selected = true
		selected = append(selected, repository)
	}
	sortRepositories(selected)

	return selected, nil
}

func idSet(ids []int64) map[int64]bool {
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func unique(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

func sortRepositories(repositories []Repository) {
	sort.Slice(repositories, func(i, j int) bool {
		return repositories[i].FullName < repositories[j].FullName
	})
}
