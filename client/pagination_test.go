package youtrack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
)

// projectPageServer serves `total` projects, honouring $top and $skip.
func projectPageServer(t *testing.T, total int, requests *int32) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests != nil {
			atomic.AddInt32(requests, 1)
		}

		top, _ := strconv.Atoi(r.URL.Query().Get("$top"))
		skip, _ := strconv.Atoi(r.URL.Query().Get("$skip"))

		page := []Project{}
		for i := skip; i < skip+top && i < total; i++ {
			page = append(page, Project{ID: fmt.Sprintf("0-%d", i)})
		}

		if err := json.NewEncoder(w).Encode(page); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
}

func TestListAllPagesToExhaustion(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		wantPages int32
	}{
		{name: "empty", total: 0, wantPages: 1},
		{name: "single partial page", total: 42, wantPages: 1},
		{name: "exactly one full page", total: defaultPageSize, wantPages: 2},
		{name: "several pages", total: 250, wantPages: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests int32
			srv := projectPageServer(t, tt.total, &requests)
			defer srv.Close()

			c, err := NewClient(srv.URL, "perm:abc")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			all, err := c.ListAllProjects(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(all) != tt.total {
				t.Fatalf("got %d projects, want %d", len(all), tt.total)
			}
			if got := atomic.LoadInt32(&requests); got != tt.wantPages {
				t.Errorf("made %d requests, want %d", got, tt.wantPages)
			}

			for i, p := range all {
				if want := fmt.Sprintf("0-%d", i); p.ID != want {
					t.Fatalf("project %d = %q, want %q (ordering or paging is wrong)", i, p.ID, want)
				}
			}
		})
	}
}

func TestListAllPropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "perm:abc")

	_, err := c.ListAllProjects(context.Background())
	if err == nil {
		t.Fatal("expected the error to propagate")
	}
	if !IsForbidden(err) {
		t.Errorf("expected a forbidden error, got %v", err)
	}
}

func TestListAllHonoursContext(t *testing.T) {
	srv := projectPageServer(t, 1000, nil)
	defer srv.Close()

	c, _ := NewClient(srv.URL, "perm:abc")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.ListAllProjects(ctx); err == nil {
		t.Fatal("expected a cancelled context to stop the walk")
	}
}

func TestListAllStopsIfServerIgnoresSkip(t *testing.T) {
	// A server that ignores $skip returns a full page forever. The walk must
	// give up rather than loop until the caller's context expires.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		page := make([]Project, defaultPageSize)
		for i := range page {
			page[i] = Project{ID: "same"}
		}
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "perm:abc")

	_, err := c.ListAllProjects(context.Background())
	if err == nil {
		t.Fatal("expected the safety limit to stop the walk")
	}
}
