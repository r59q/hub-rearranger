package sqlite

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSelectionStorePersistsAndReplacesSelection(t *testing.T) {
	// Arrange.
	path := filepath.Join(t.TempDir(), "repositories.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	// Act.
	if err := store.Replace(context.Background(), []int64{3, 8}); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	if err := reopened.Replace(context.Background(), []int64{5}); err != nil {
		t.Fatalf("second Replace() error = %v", err)
	}
	ids, err := reopened.List(context.Background())

	// Assert.
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !reflect.DeepEqual(ids, []int64{5}) {
		t.Fatalf("ids = %v, want [5]", ids)
	}
}
