// Package history is a small local store of recently-accessed projects and
// branches, keyed by profile. It's the persistence layer behind the "recent
// projects" and "recent branches" dashboard cards, and is deliberately
// best-effort: a read or write failure never blocks the rest of the app.
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// maxEntries bounds each recency list. Old entries fall off the end.
const maxEntries = 10

// Store is the whole on-disk history file: one bucket per profile.
type Store struct {
	Profiles map[string]*ProfileHistory `json:"profiles"`
}

// ProfileHistory is one profile's recency lists, most-recent first.
type ProfileHistory struct {
	Projects []Project `json:"projects,omitempty"`
	Branches []Branch  `json:"branches,omitempty"`
}

// Project is a recently-accessed GitLab project.
type Project struct {
	Path       string    `json:"path"`
	Name       string    `json:"name,omitempty"`
	LastAccess time.Time `json:"last_access"`
}

// Branch is a recently-accessed branch, with its merge request if one exists.
type Branch struct {
	Name        string    `json:"name"`
	ProjectPath string    `json:"project_path"`
	MRIID       int       `json:"mr_iid,omitempty"`
	MRTitle     string    `json:"mr_title,omitempty"`
	LastAccess  time.Time `json:"last_access"`
}

// Load reads the store at path. A missing file yields an empty store rather
// than an error — there's simply no history yet.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{Profiles: map[string]*ProfileHistory{}}, nil
		}
		return nil, fmt.Errorf("reading history at %s: %w", path, err)
	}

	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("history at %s is not valid JSON: %w", path, err)
	}
	if s.Profiles == nil {
		s.Profiles = map[string]*ProfileHistory{}
	}
	return &s, nil
}

// Save writes the store to path, creating the parent directory as needed. It
// mirrors config's I/O conventions (0700 dir, 0600 file) since both live under
// ~/.config/gitlab-tui.
func Save(path string, s *Store) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating history directory: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding history: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing history to %s: %w", path, err)
	}
	return nil
}

// RecordProject moves p to the front of profile's recent-projects list,
// de-duplicating by path and bounding the list to maxEntries.
func (s *Store) RecordProject(profile string, p Project) {
	bucket := s.bucket(profile)
	bucket.Projects = prependBounded(bucket.Projects, p, func(x Project) bool {
		return x.Path == p.Path
	})
}

// RecordBranch moves b to the front of profile's recent-branches list,
// de-duplicating by project+name and bounding the list to maxEntries.
func (s *Store) RecordBranch(profile string, b Branch) {
	bucket := s.bucket(profile)
	bucket.Branches = prependBounded(bucket.Branches, b, func(x Branch) bool {
		return x.ProjectPath == b.ProjectPath && x.Name == b.Name
	})
}

// Projects returns profile's recent projects, most-recent first.
func (s *Store) Projects(profile string) []Project {
	if bucket, ok := s.Profiles[profile]; ok {
		return bucket.Projects
	}
	return nil
}

// Branches returns profile's recent branches, most-recent first.
func (s *Store) Branches(profile string) []Branch {
	if bucket, ok := s.Profiles[profile]; ok {
		return bucket.Branches
	}
	return nil
}

func (s *Store) bucket(profile string) *ProfileHistory {
	if s.Profiles == nil {
		s.Profiles = map[string]*ProfileHistory{}
	}
	bucket, ok := s.Profiles[profile]
	if !ok {
		bucket = &ProfileHistory{}
		s.Profiles[profile] = bucket
	}
	return bucket
}

// prependBounded puts item at the front, drops any prior entry sameKey matches,
// and caps the result at maxEntries.
func prependBounded[T any](list []T, item T, sameKey func(T) bool) []T {
	out := make([]T, 0, len(list)+1)
	out = append(out, item)
	for _, existing := range list {
		if !sameKey(existing) {
			out = append(out, existing)
		}
	}
	if len(out) > maxEntries {
		out = out[:maxEntries]
	}
	return out
}
