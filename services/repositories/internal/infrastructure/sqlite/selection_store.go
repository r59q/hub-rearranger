package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type SelectionStore struct {
	db *sql.DB
}

func Open(path string) (*SelectionStore, error) {
	db, err := sql.Open("sqlite", filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &SelectionStore{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SelectionStore) Close() error {
	return s.db.Close()
}

func (s *SelectionStore) migrate(ctx context.Context) error {
	const schema = `
		PRAGMA journal_mode = WAL;
		PRAGMA foreign_keys = ON;
		CREATE TABLE IF NOT EXISTS selected_repositories (
			repository_id INTEGER PRIMARY KEY CHECK (repository_id > 0),
			selected_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate SQLite database: %w", err)
	}
	return nil
}

func (s *SelectionStore) List(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT repository_id FROM selected_repositories ORDER BY selected_at, repository_id`)
	if err != nil {
		return nil, fmt.Errorf("query selected repositories: %w", err)
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan selected repository: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate selected repositories: %w", err)
	}
	return ids, nil
}

func (s *SelectionStore) Replace(ctx context.Context, ids []int64) error {
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin selection transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	if _, err := transaction.ExecContext(ctx, `DELETE FROM selected_repositories`); err != nil {
		return fmt.Errorf("clear repository selection: %w", err)
	}
	for _, id := range ids {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO selected_repositories (repository_id) VALUES (?)`, id); err != nil {
			return fmt.Errorf("insert selected repository: %w", err)
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit repository selection: %w", err)
	}
	return nil
}
