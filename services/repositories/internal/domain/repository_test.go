package domain

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeCatalog struct {
	repositories []Repository
	err          error
}

func (f *fakeCatalog) ListForAccount(context.Context) ([]Repository, error) {
	return append([]Repository(nil), f.repositories...), f.err
}

type fakeSelectionStore struct {
	ids      []int64
	replaced []int64
	err      error
}

func (f *fakeSelectionStore) List(context.Context) ([]int64, error) {
	return append([]int64(nil), f.ids...), f.err
}

func (f *fakeSelectionStore) Replace(_ context.Context, ids []int64) error {
	f.replaced = append([]int64(nil), ids...)
	return f.err
}

func TestListAvailableMarksSelectedAndSorts(t *testing.T) {
	// Arrange.
	catalog := &fakeCatalog{repositories: []Repository{
		{ID: 2, FullName: "octo/zebra"},
		{ID: 1, FullName: "octo/alpha"},
	}}
	store := &fakeSelectionStore{ids: []int64{2}}
	service := NewRepositoryService(catalog, store)

	// Act.
	repositories, err := service.ListAvailable(context.Background())

	// Assert.
	if err != nil {
		t.Fatalf("ListAvailable() error = %v", err)
	}
	if got := []string{repositories[0].FullName, repositories[1].FullName}; !reflect.DeepEqual(got, []string{"octo/alpha", "octo/zebra"}) {
		t.Fatalf("repository order = %v", got)
	}
	if repositories[0].Selected || !repositories[1].Selected {
		t.Fatalf("selected flags = [%v, %v]", repositories[0].Selected, repositories[1].Selected)
	}
}

func TestReplaceSelectionValidatesAndDeduplicates(t *testing.T) {
	// Arrange.
	catalog := &fakeCatalog{repositories: []Repository{{ID: 1, FullName: "octo/alpha"}, {ID: 2, FullName: "octo/beta"}}}
	store := &fakeSelectionStore{}
	service := NewRepositoryService(catalog, store)

	// Act.
	selected, err := service.ReplaceSelection(context.Background(), []int64{2, 2, 1})

	// Assert.
	if err != nil {
		t.Fatalf("ReplaceSelection() error = %v", err)
	}
	if !reflect.DeepEqual(store.replaced, []int64{2, 1}) {
		t.Fatalf("stored IDs = %v", store.replaced)
	}
	if got := []string{selected[0].FullName, selected[1].FullName}; !reflect.DeepEqual(got, []string{"octo/alpha", "octo/beta"}) {
		t.Fatalf("selected repositories = %v", got)
	}
}

func TestReplaceSelectionRejectsUnknownRepository(t *testing.T) {
	// Arrange.
	service := NewRepositoryService(
		&fakeCatalog{repositories: []Repository{{ID: 1}}},
		&fakeSelectionStore{},
	)

	// Act.
	_, err := service.ReplaceSelection(context.Background(), []int64{7})

	// Assert.
	var unknown *UnknownRepositoriesError
	if !errors.As(err, &unknown) || !reflect.DeepEqual(unknown.IDs, []int64{7}) {
		t.Fatalf("error = %v, want unknown repository 7", err)
	}
}
