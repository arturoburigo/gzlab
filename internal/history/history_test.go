package history

import (
	"path/filepath"
	"testing"
	"time"
)

func at(min int) time.Time {
	return time.Date(2026, 7, 2, 10, min, 0, 0, time.UTC)
}

func TestRecordProject_RecencyDedupAndBound(t *testing.T) {
	s := &Store{}
	for i := 0; i < maxEntries+5; i++ {
		s.RecordProject("empresa", Project{Path: pathN(i), LastAccess: at(i)})
	}

	got := s.Projects("empresa")
	if len(got) != maxEntries {
		t.Fatalf("len = %d, want %d (bounded)", len(got), maxEntries)
	}
	// most-recent first: the last recorded is at the front
	if got[0].Path != pathN(maxEntries+4) {
		t.Errorf("front = %q, want the most recently recorded", got[0].Path)
	}

	// re-recording an existing path moves it to the front without growing
	s.RecordProject("empresa", Project{Path: got[3].Path, LastAccess: at(99)})
	moved := s.Projects("empresa")
	if len(moved) != maxEntries {
		t.Errorf("len after re-record = %d, want %d (dedup, no growth)", len(moved), maxEntries)
	}
	if moved[0].Path != got[3].Path {
		t.Errorf("front = %q, want the re-recorded path moved to front", moved[0].Path)
	}
	if countPath(moved, got[3].Path) != 1 {
		t.Errorf("re-recorded path appears %d times, want 1", countPath(moved, got[3].Path))
	}
}

func TestRecordBranch_KeyedByProjectAndName(t *testing.T) {
	s := &Store{}
	s.RecordBranch("empresa", Branch{Name: "feature", ProjectPath: "a/b", MRIID: 1, LastAccess: at(1)})
	s.RecordBranch("empresa", Branch{Name: "feature", ProjectPath: "c/d", MRIID: 2, LastAccess: at(2)})
	// same name, different project -> distinct entries
	if got := len(s.Branches("empresa")); got != 2 {
		t.Fatalf("len = %d, want 2 (same branch name in different projects are distinct)", got)
	}

	// same project+name -> updates in place, moving to front with new MR info
	s.RecordBranch("empresa", Branch{Name: "feature", ProjectPath: "a/b", MRIID: 7, MRTitle: "updated", LastAccess: at(3)})
	got := s.Branches("empresa")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (re-record dedups)", len(got))
	}
	if got[0].ProjectPath != "a/b" || got[0].MRIID != 7 || got[0].MRTitle != "updated" {
		t.Errorf("front = %+v, want the updated a/b entry with MR 7", got[0])
	}
}

func TestStore_ProfilesAreIsolated(t *testing.T) {
	s := &Store{}
	s.RecordProject("empresa", Project{Path: "a/b", LastAccess: at(1)})
	s.RecordProject("pessoal", Project{Path: "c/d", LastAccess: at(2)})

	if got := s.Projects("empresa"); len(got) != 1 || got[0].Path != "a/b" {
		t.Errorf("empresa projects = %+v, want just a/b", got)
	}
	if got := s.Projects("pessoal"); len(got) != 1 || got[0].Path != "c/d" {
		t.Errorf("pessoal projects = %+v, want just c/d", got)
	}
	if got := s.Projects("unknown"); got != nil {
		t.Errorf("unknown profile projects = %+v, want nil", got)
	}
}

func TestLoadSave_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "history.json")

	s := &Store{}
	s.RecordProject("empresa", Project{Path: "a/b", Name: "b", LastAccess: at(1)})
	s.RecordBranch("empresa", Branch{Name: "feature", ProjectPath: "a/b", MRIID: 42, MRTitle: "t", LastAccess: at(2)})
	if err := Save(path, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := loaded.Projects("empresa"); len(got) != 1 || got[0].Path != "a/b" || got[0].Name != "b" {
		t.Errorf("round-tripped projects = %+v", got)
	}
	branches := loaded.Branches("empresa")
	if len(branches) != 1 || branches[0].MRIID != 42 || branches[0].MRTitle != "t" {
		t.Errorf("round-tripped branches = %+v", branches)
	}
}

func TestLoad_MissingFileIsEmptyStore(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("Load of a missing file should not error: %v", err)
	}
	if s == nil || s.Profiles == nil {
		t.Fatal("expected an empty, usable store")
	}
	if got := s.Projects("anything"); got != nil {
		t.Errorf("empty store Projects = %+v, want nil", got)
	}
}

func pathN(n int) string {
	return "ns/project-" + string(rune('a'+n))
}

func countPath(list []Project, path string) int {
	n := 0
	for _, p := range list {
		if p.Path == path {
			n++
		}
	}
	return n
}
