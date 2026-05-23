package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Checkpoint struct {
	LastModified string
	LastPollAt   time.Time
}

func defaultPath() (string, error) {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "state")
	}
	dir = filepath.Join(dir, "ghlistend")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.db"), nil
}

func Open() (*Store, error) {
	path, err := defaultPath()
	if err != nil {
		return nil, err
	}
	return OpenAt(path)
}

func OpenAt(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS checkpoint (
			id INTEGER PRIMARY KEY CHECK (id=1),
			last_modified TEXT,
			last_poll_at  TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS seen (
			thread_id   TEXT NOT NULL,
			updated_at  TEXT NOT NULL,
			notified_at TEXT NOT NULL,
			PRIMARY KEY (thread_id, updated_at)
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func (s *Store) LoadCheckpoint() (Checkpoint, error) {
	var cp Checkpoint
	var lm sql.NullString
	var lp sql.NullString
	row := s.db.QueryRow(`SELECT last_modified, last_poll_at FROM checkpoint WHERE id=1`)
	err := row.Scan(&lm, &lp)
	if errors.Is(err, sql.ErrNoRows) {
		return cp, nil
	}
	if err != nil {
		return cp, err
	}
	if lm.Valid {
		cp.LastModified = lm.String
	}
	if lp.Valid {
		if t, err := time.Parse(time.RFC3339, lp.String); err == nil {
			cp.LastPollAt = t
		}
	}
	return cp, nil
}

func (s *Store) SaveCheckpoint(lastModified string, lastPollAt time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO checkpoint(id, last_modified, last_poll_at) VALUES(1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET last_modified=excluded.last_modified, last_poll_at=excluded.last_poll_at`,
		lastModified, lastPollAt.UTC().Format(time.RFC3339))
	return err
}

func (s *Store) HasCheckpoint() (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM checkpoint WHERE id=1`).Scan(&n)
	return n > 0, err
}

func (s *Store) Seen(threadID, updatedAt string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT 1 FROM seen WHERE thread_id=? AND updated_at=? LIMIT 1`,
		threadID, updatedAt).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) MarkSeen(threadID, updatedAt string) error {
	_, err := s.db.Exec(`INSERT OR IGNORE INTO seen(thread_id, updated_at, notified_at) VALUES(?, ?, ?)`,
		threadID, updatedAt, time.Now().UTC().Format(time.RFC3339))
	return err
}
