package store

import (
	"path/filepath"
	"testing"
	"time"
)

// Feature tests exercise a real backing system (here: a real sqlite file)
// but stay scoped to one package. `t.TempDir()` gives us an auto-cleaned
// temp directory so tests can't pollute each other or leak state.

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := OpenAt(path)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestCheckpointRoundtrip(t *testing.T) {
	s := newTestStore(t)

	if has, _ := s.HasCheckpoint(); has {
		t.Fatal("fresh store should have no checkpoint")
	}

	now := time.Now().UTC().Truncate(time.Second)
	if err := s.SaveCheckpoint("Wed, 22 May 2026 12:00:00 GMT", now); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}

	cp, err := s.LoadCheckpoint()
	if err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}
	if cp.LastModified != "Wed, 22 May 2026 12:00:00 GMT" {
		t.Errorf("LastModified = %q", cp.LastModified)
	}
	if !cp.LastPollAt.Equal(now) {
		t.Errorf("LastPollAt = %v, want %v", cp.LastPollAt, now)
	}

	has, _ := s.HasCheckpoint()
	if !has {
		t.Error("HasCheckpoint should be true after save")
	}
}

func TestCheckpointUpsert(t *testing.T) {
	s := newTestStore(t)
	_ = s.SaveCheckpoint("first", time.Now())
	_ = s.SaveCheckpoint("second", time.Now())

	cp, _ := s.LoadCheckpoint()
	if cp.LastModified != "second" {
		t.Errorf("expected upsert to overwrite, got %q", cp.LastModified)
	}
}

func TestSeenIdempotent(t *testing.T) {
	s := newTestStore(t)

	if seen, _ := s.Seen("thread-1", "2026-05-22T10:00:00Z"); seen {
		t.Fatal("unseen thread reported as seen")
	}

	if err := s.MarkSeen("thread-1", "2026-05-22T10:00:00Z"); err != nil {
		t.Fatalf("MarkSeen: %v", err)
	}
	// duplicate insert must not error (INSERT OR IGNORE)
	if err := s.MarkSeen("thread-1", "2026-05-22T10:00:00Z"); err != nil {
		t.Fatalf("duplicate MarkSeen: %v", err)
	}

	if seen, _ := s.Seen("thread-1", "2026-05-22T10:00:00Z"); !seen {
		t.Error("marked thread reported as unseen")
	}

	// Different updated_at on the same thread should be treated as new — this
	// is the whole reason we key by (thread_id, updated_at) per the spec.
	if seen, _ := s.Seen("thread-1", "2026-05-22T11:00:00Z"); seen {
		t.Error("bumped updated_at should not be seen yet")
	}
}
