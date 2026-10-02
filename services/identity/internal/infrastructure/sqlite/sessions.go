package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

func (s *Store) PutSession(ctx context.Context, id string, session domain.Session) error {
	if err := s.purge(ctx); err != nil {
		return err
	}

	data, err := s.encode("session", id, session)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `INSERT INTO sessions(id,data,expires) VALUES(?,?,?)`, id, data, session.Expires.Unix())
	if err != nil {
		return domain.ErrUnavailable
	}
	return nil
}

func (s *Store) WithSession(ctx context.Context, id string, operation func(*domain.Session) error) error {
	// A single identity replica owns this connection. Keeping the transaction
	// through the operation serializes token rotation and sign-out with writes.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ErrUnavailable
	}
	defer func() { _ = tx.Rollback() }()

	var data []byte
	err = tx.QueryRowContext(ctx, `SELECT data FROM sessions WHERE id=?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrReconnect
	}
	if err != nil {
		return domain.ErrUnavailable
	}

	var session domain.Session
	result := s.decode("session", id, data, &session)
	if result == nil {
		result = operation(&session)
	}

	if errors.Is(result, domain.ErrReconnect) || session.Credentials.Access == "" {
		_, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE id=?`, id)
	} else {
		// Preserve a rotated token even if a subsequent repository check denies access.
		// GitHub invalidates the old pair at refresh, so rolling it back loses the session.
		data, err = s.encode("session", id, session)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE sessions SET data=? WHERE id=?`, data, id)
		}
	}

	if err != nil || tx.Commit() != nil {
		return domain.ErrUnavailable
	}
	return result
}
