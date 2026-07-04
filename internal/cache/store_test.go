package cache

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreGetSetHonorsTTL(t *testing.T) {
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	store := NewStore(t.TempDir())
	store.now = func() time.Time { return now }

	if err := store.Set("answer", time.Minute, map[string]int{"value": 42}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	var got map[string]int
	if ok := store.Get("answer", &got); !ok {
		t.Fatal("Get() missed fresh cache entry")
	}
	if got["value"] != 42 {
		t.Errorf("cached value = %v, want 42", got["value"])
	}

	now = now.Add(time.Minute)
	if ok := store.Get("answer", &got); ok {
		t.Fatal("Get() hit expired cache entry")
	}
}

func TestClearRemovesCacheDirectory(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.Set("x", time.Minute, "cached"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if err := Clear(root); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if matches, err := filepath.Glob(filepath.Join(root, "*")); err != nil {
		t.Fatalf("Glob() error = %v", err)
	} else if len(matches) != 0 {
		t.Fatalf("cache directory still has entries: %v", matches)
	}
}
