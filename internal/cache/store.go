package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Store persists small JSON-serializable values as individual files.
type Store struct {
	root string
	now  func() time.Time
}

type entry struct {
	ExpiresAt time.Time       `json:"expires_at"`
	Value     json.RawMessage `json:"value"`
}

func NewStore(root string) *Store {
	return &Store{root: root, now: time.Now}
}

func (s *Store) Get(key string, dst any) bool {
	if s == nil || s.root == "" {
		return false
	}
	data, err := os.ReadFile(s.path(key))
	if err != nil {
		return false
	}

	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		_ = os.Remove(s.path(key))
		return false
	}
	if !e.ExpiresAt.IsZero() && !s.now().Before(e.ExpiresAt) {
		_ = os.Remove(s.path(key))
		return false
	}
	if err := json.Unmarshal(e.Value, dst); err != nil {
		_ = os.Remove(s.path(key))
		return false
	}
	return true
}

func (s *Store) Set(key string, ttl time.Duration, value any) error {
	if s == nil || s.root == "" || ttl <= 0 {
		return nil
	}
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding cache value: %w", err)
	}
	e := entry{
		ExpiresAt: s.now().Add(ttl),
		Value:     valueJSON,
	}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding cache entry: %w", err)
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}
	if err := os.Chmod(s.root, 0o700); err != nil {
		return fmt.Errorf("securing cache directory: %w", err)
	}

	path := s.path(key)
	tmp, err := os.CreateTemp(s.root, ".tmp-*")
	if err != nil {
		return fmt.Errorf("creating cache temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing cache temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing cache temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("securing cache temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("writing cache file: %w", err)
	}
	return nil
}

func Clear(root string) error {
	if root == "" {
		return nil
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("clearing cache at %s: %w", root, err)
	}
	return nil
}

func (s *Store) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.root, hex.EncodeToString(sum[:])+".json")
}
