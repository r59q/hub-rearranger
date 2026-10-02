// Package sqlite owns encrypted identity persistence. It never shares its database.
package sqlite

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
	_ "modernc.org/sqlite"
)

type Store struct {
	db     *sql.DB
	cipher cipher.AEAD
}

func Open(path string, key []byte) (*Store, error) {
	if len(key) != 32 {
		return nil, domain.ErrUnavailable
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, domain.ErrUnavailable
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	_ = file.Close()
	if err := os.Chmod(path, 0600); err != nil {
		return nil, domain.ErrUnavailable
	}

	db, err := sql.Open("sqlite", filepath.Clean(path))
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	db.SetMaxOpenConns(1)

	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	store := &Store{db: db, cipher: aead}

	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS intents (id TEXT PRIMARY KEY, data BLOB NOT NULL, expires INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY, data BLOB NOT NULL, expires INTEGER NOT NULL);`)
	if err != nil {
		_ = db.Close()
		return nil, domain.ErrUnavailable
	}

	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) encode(kind, id string, value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, domain.ErrUnavailable
	}

	nonce := make([]byte, s.cipher.NonceSize())
	_, _ = rand.Read(nonce)
	return s.cipher.Seal(nonce, nonce, data, []byte("identity/"+kind+"/v1/"+id)), nil
}

func (s *Store) decode(kind, id string, data []byte, value any) error {
	size := s.cipher.NonceSize()
	if len(data) < size {
		return domain.ErrReconnect
	}

	plain, err := s.cipher.Open(nil, data[:size], data[size:], []byte("identity/"+kind+"/v1/"+id))
	if err != nil || json.Unmarshal(plain, value) != nil {
		return domain.ErrReconnect
	}
	return nil
}

func (s *Store) purge(ctx context.Context) error {
	for _, table := range []string{"intents", "sessions"} {
		if _, err := s.db.ExecContext(ctx, "DELETE FROM "+table+" WHERE expires <= ?", time.Now().Unix()); err != nil {
			return domain.ErrUnavailable
		}
	}
	return nil
}
