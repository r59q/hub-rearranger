package sqlite

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

func (s *Store) PutIntent(ctx context.Context, id string, intent domain.Intent) error {
	if err := s.purge(ctx); err != nil {
		return err
	}

	data, err := s.encode("intent", id, intent)
	if err != nil {
		return err
	}

	// Bound anonymous pending sign-ins. Expired rows are purged before this check.
	result, err := s.db.ExecContext(ctx, `INSERT INTO intents(id,data,expires) SELECT ?,?,? WHERE (SELECT count(*) FROM intents) < 10000`, id, data, intent.Expires.Unix())
	if err != nil {
		return domain.ErrUnavailable
	}

	count, _ := result.RowsAffected()
	if count != 1 {
		return domain.ErrUnavailable
	}
	return nil
}

func (s *Store) TakeIntent(ctx context.Context, id, binding string) (domain.Intent, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, `DELETE FROM intents WHERE id=? RETURNING data`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Intent{}, domain.ErrForgery
	}
	if err != nil {
		return domain.Intent{}, domain.ErrUnavailable
	}

	var intent domain.Intent
	if s.decode("intent", id, data, &intent) != nil || subtle.ConstantTimeCompare([]byte(intent.Binding), []byte(binding)) != 1 {
		return domain.Intent{}, domain.ErrForgery
	}
	return intent, nil
}
