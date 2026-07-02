package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient starts an httptest server driven by handler and returns a
// Client pointed at it.
func newTestClient(t *testing.T, handler http.HandlerFunc) Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL, "test-token")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()
	w.WriteHeader(status)
	if body != nil {
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
	}
}

func TestRealClient_CurrentUser(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/user" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		writeJSON(t, w, http.StatusOK, map[string]any{"id": 735, "username": "arturo.burigo", "name": "Arturo Burigo"})
	})

	got, err := client.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("CurrentUser() error = %v", err)
	}
	if got.ID != 735 || got.Username != "arturo.burigo" {
		t.Errorf("CurrentUser() = %+v", got)
	}
}

func TestRealClient_GetProjectByPath_NotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, map[string]any{"message": "404 Project Not Found"})
	})

	_, err := client.GetProjectByPath(context.Background(), "team/service")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetProjectByPath() error = %v, want ErrNotFound", err)
	}
}

func TestRealClient_GetProjectByPath_Success(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, map[string]any{
			"id": 2087, "path_with_namespace": "atendimento/protocolo/cadastros/api-protocolo-cadastros",
			"name": "api-protocolo-cadastros", "default_branch": "master",
		})
	})

	got, err := client.GetProjectByPath(context.Background(), "atendimento/protocolo/cadastros/api-protocolo-cadastros")
	if err != nil {
		t.Fatalf("GetProjectByPath() error = %v", err)
	}
	if got.ID != 2087 || got.DefaultBranch != "master" {
		t.Errorf("GetProjectByPath() = %+v", got)
	}
}

func TestRealClient_ListMergeRequests_DefaultsToOpenedState(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("state"); got != "opened" {
			t.Errorf("state query param = %q, want %q", got, "opened")
		}
		writeJSON(t, w, http.StatusOK, []map[string]any{
			{"iid": 251, "title": "Alinha ao commons", "state": "opened"},
		})
	})

	got, err := client.ListMergeRequests(context.Background(), 2087, ListMergeRequestsOptions{})
	if err != nil {
		t.Fatalf("ListMergeRequests() error = %v", err)
	}
	if len(got) != 1 || got[0].IID != 251 {
		t.Errorf("ListMergeRequests() = %+v", got)
	}
}

func TestRealClient_ListMergeRequests_FiltersBySourceBranch(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("source_branch"); got != "bugfix-PD-26527" {
			t.Errorf("source_branch query param = %q, want %q", got, "bugfix-PD-26527")
		}
		writeJSON(t, w, http.StatusOK, []map[string]any{})
	})

	if _, err := client.ListMergeRequests(context.Background(), 2087, ListMergeRequestsOptions{SourceBranch: "bugfix-PD-26527"}); err != nil {
		t.Fatalf("ListMergeRequests() error = %v", err)
	}
}

func TestRealClient_GetMergeRequest_ApprovalsBestEffort(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/approvals"):
			// Simulate a Free-tier instance where approvals are unavailable.
			writeJSON(t, w, http.StatusForbidden, map[string]any{"message": "403 Forbidden"})
		default:
			writeJSON(t, w, http.StatusOK, map[string]any{
				"iid": 251, "title": "Alinha ao commons", "state": "opened",
				"pipeline": map[string]any{"id": 3237626, "status": "failed", "ref": "refs/merge-requests/251/head"},
			})
		}
	})

	got, err := client.GetMergeRequest(context.Background(), 2087, 251)
	if err != nil {
		t.Fatalf("GetMergeRequest() error = %v (approvals failure should not be fatal)", err)
	}
	if got.Pipeline == nil || got.Pipeline.Status != PipelineStatusFailed {
		t.Errorf("Pipeline = %+v, want status failed", got.Pipeline)
	}
	if got.ApprovalsRequired != 0 {
		t.Errorf("ApprovalsRequired = %d, want 0 (approvals endpoint failed)", got.ApprovalsRequired)
	}
}

func TestRealClient_GetMergeRequest_NotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, map[string]any{"message": "404 Not found"})
	})

	_, err := client.GetMergeRequest(context.Background(), 2087, 999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetMergeRequest() error = %v, want ErrNotFound", err)
	}
}

func TestRealClient_FindMergeRequestForBranch_Found(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/approvals"):
			writeJSON(t, w, http.StatusOK, map[string]any{"approvals_required": 2, "approvals_left": 0})
		case strings.Contains(r.URL.Path, "/merge_requests/251"):
			writeJSON(t, w, http.StatusOK, map[string]any{"iid": 251, "title": "Alinha ao commons", "state": "opened"})
		default:
			// list endpoint
			writeJSON(t, w, http.StatusOK, []map[string]any{{"iid": 251, "title": "Alinha ao commons", "state": "opened"}})
		}
	})

	got, err := client.FindMergeRequestForBranch(context.Background(), 2087, "bugfix-PD-26527")
	if err != nil {
		t.Fatalf("FindMergeRequestForBranch() error = %v", err)
	}
	if got.IID != 251 || got.ApprovalsRequired != 2 || got.ApprovalsGiven != 2 {
		t.Errorf("FindMergeRequestForBranch() = %+v", got)
	}
}

func TestRealClient_FindMergeRequestForBranch_NotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, []map[string]any{})
	})

	_, err := client.FindMergeRequestForBranch(context.Background(), 2087, "main")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("FindMergeRequestForBranch() error = %v, want ErrNotFound", err)
	}
}
