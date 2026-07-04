package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Store struct {
	Profiles map[string][]Workspace `json:"profiles"`
}

type Workspace struct {
	Name          string            `json:"name"`
	Profile       string            `json:"profile"`
	MergeRequests []MergeRequestRef `json:"merge_requests"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

type MergeRequestRef struct {
	ProjectPath string `json:"project"`
	ProjectID   int    `json:"project_id"`
	IID         int    `json:"iid"`
	Title       string `json:"title,omitempty"`
	WebURL      string `json:"web_url,omitempty"`
	Status      string `json:"status,omitempty"`
	Pipeline    string `json:"pipeline,omitempty"`
	Approvals   string `json:"approvals,omitempty"`
	Branch      string `json:"branch,omitempty"`
}

func Load(path string) (*Store, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{Profiles: map[string][]Workspace{}}, nil
		}
		return nil, err
	}
	var store Store
	if err := json.Unmarshal(b, &store); err != nil {
		return nil, err
	}
	if store.Profiles == nil {
		store.Profiles = map[string][]Workspace{}
	}
	return &store, nil
}

func Save(path string, store *Store) error {
	if store == nil {
		store = &Store{Profiles: map[string][]Workspace{}}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

func (s *Store) List(profile string) []Workspace {
	if s == nil {
		return nil
	}
	items := append([]Workspace(nil), s.Profiles[profile]...)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items
}

func (s *Store) UpsertMR(profile, name string, ref MergeRequestRef) Workspace {
	if s.Profiles == nil {
		s.Profiles = map[string][]Workspace{}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "default"
	}
	items := s.Profiles[profile]
	idx := -1
	for i := range items {
		if items[i].Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		items = append(items, Workspace{Name: name, Profile: profile})
		idx = len(items) - 1
	}
	ws := &items[idx]
	ws.Profile = profile
	ws.UpdatedAt = time.Now()
	found := false
	for i := range ws.MergeRequests {
		if ws.MergeRequests[i].ProjectPath == ref.ProjectPath && ws.MergeRequests[i].IID == ref.IID {
			ws.MergeRequests[i] = ref
			found = true
			break
		}
	}
	if !found {
		ws.MergeRequests = append(ws.MergeRequests, ref)
	}
	s.Profiles[profile] = items
	return *ws
}

func (s *Store) RemoveMR(profile, name, projectPath string, iid int) {
	items := s.Profiles[profile]
	for i := range items {
		if items[i].Name != name {
			continue
		}
		refs := items[i].MergeRequests[:0]
		for _, ref := range items[i].MergeRequests {
			if ref.ProjectPath == projectPath && ref.IID == iid {
				continue
			}
			refs = append(refs, ref)
		}
		items[i].MergeRequests = refs
		items[i].UpdatedAt = time.Now()
		break
	}
	s.Profiles[profile] = items
}
