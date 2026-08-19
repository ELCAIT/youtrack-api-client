package youtrack

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const (
	testAppID       = "145-92"
	testAppName     = "Test App"
	testAppUsageID  = "usage-1"
	testAppsPath    = "/api/admin/apps"
	testAppPath     = "/api/admin/apps/145-92"
	testUsagesPath  = "/api/admin/apps/145-92/usages"
	testUsagePath   = "/api/admin/apps/145-92/usages/usage-1"
	testProjectsAPI = "/api/admin/projects"

	fmtUnexpectedPath   = "unexpected path: %s"
	fmtUnexpectedMethod = "unexpected method: got %s, want %s"
)

func testAppUsage(enabled bool) AppUsage {
	return AppUsage{ID: testAppUsageID, Enabled: enabled, Project: &Project{ID: testProjectID}}
}

func TestListApps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
		wantLen int
	}{
		{
			name: "returns apps list",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodGet {
					t.Errorf(fmtUnexpectedMethod, r.Method, httpMethodGet)
				}
				if r.URL.Path != testAppsPath {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}
				if got := r.URL.Query().Get("$top"); got != "10" {
					t.Errorf("unexpected $top: got %s, want 10", got)
				}
				encodeJSON(t, w, []App{
					{ID: testAppID, Name: testAppName, Enabled: true},
					{ID: "145-93", Name: "Other App"},
				})
			},
			wantLen: 2,
		},
		{
			name: testCaseServerFailure,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
		{
			name: "returns error on invalid json",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte(testInvalidJSON)); err != nil {
					t.Errorf(fmtUnexpectedError, err)
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, tc.handler)
			defer server.Close()

			got, err := client.ListApps(context.Background(), 10, 0)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if len(got) != tc.wantLen {
				t.Fatalf("unexpected apps count: got %d, want %d", len(got), tc.wantLen)
			}
		})
	}
}

func TestGetAppByID(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testAppPath {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		encodeJSON(t, w, App{ID: testAppID, Name: testAppName, Enabled: true, AppType: "APP", Version: "1.0.0"})
	})
	defer server.Close()

	got, err := client.GetAppByID(context.Background(), testAppID)
	if checkErr(t, err, false) {
		return
	}
	if got.ID != testAppID {
		t.Fatalf(fmtUnexpectedID, got.ID, testAppID)
	}
	if !got.Enabled {
		t.Fatal("expected app to be enabled")
	}
}

func TestGetAppByName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		lookup  string
		apps    []App
		wantErr bool
		wantID  string
	}{
		{
			name:   "exact match",
			lookup: testAppName,
			apps:   []App{{ID: "145-93", Name: "test app"}, {ID: testAppID, Name: testAppName}},
			wantID: testAppID,
		},
		{
			name:   "case-insensitive fallback",
			lookup: testAppName,
			apps:   []App{{ID: testAppID, Name: "TEST APP"}},
			wantID: testAppID,
		},
		{
			name:    "not found",
			lookup:  "Missing App",
			apps:    []App{{ID: testAppID, Name: testAppName}},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				encodeJSON(t, w, tc.apps)
			})
			defer server.Close()

			got, err := client.GetAppByName(context.Background(), tc.lookup)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.ID != tc.wantID {
				t.Fatalf(fmtUnexpectedID, got.ID, tc.wantID)
			}
		})
	}
}

func TestListAppUsages(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testUsagesPath {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		encodeJSON(t, w, []AppUsage{testAppUsage(true)})
	})
	defer server.Close()

	got, err := client.ListAppUsages(context.Background(), testAppID)
	if checkErr(t, err, false) {
		return
	}
	if len(got) != 1 || got[0].ID != testAppUsageID {
		t.Fatalf("unexpected usages: %+v", got)
	}
}

func TestGetAppUsageForProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		usages   []AppUsage
		wantNil  bool
		wantEnab bool
	}{
		{name: "attached project", usages: []AppUsage{testAppUsage(true)}, wantEnab: true},
		{name: "other project only", usages: []AppUsage{{ID: "usage-9", Project: &Project{ID: "0-9"}}}, wantNil: true},
		{name: "no usages", usages: []AppUsage{}, wantNil: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				encodeJSON(t, w, tc.usages)
			})
			defer server.Close()

			got, err := client.GetAppUsageForProject(context.Background(), testAppID, testProjectID)
			if checkErr(t, err, false) {
				return
			}
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil usage, got %+v", got)
				}
				return
			}
			if got == nil || got.Enabled != tc.wantEnab {
				t.Fatalf("unexpected usage: %+v", got)
			}
		})
	}
}

func TestAttachAppToProject(t *testing.T) {
	t.Parallel()

	t.Run("returns created usage", func(t *testing.T) {
		t.Parallel()

		client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != httpMethodPost {
				t.Errorf(fmtUnexpectedMethod, r.Method, httpMethodPost)
			}
			var payload appUsageAttachPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("failed to decode payload: %v", err)
			}
			if payload.Project == nil || payload.Project.ID != testProjectID {
				t.Errorf("unexpected payload project: %+v", payload.Project)
			}
			encodeJSON(t, w, testAppUsage(true))
		})
		defer server.Close()

		got, err := client.AttachAppToProject(context.Background(), testAppID, testProjectID)
		if checkErr(t, err, false) {
			return
		}
		if got.ID != testAppUsageID {
			t.Fatalf(fmtUnexpectedID, got.ID, testAppUsageID)
		}
	})

	t.Run("refetches usage on empty response body", func(t *testing.T) {
		t.Parallel()

		client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == httpMethodPost {
				w.WriteHeader(http.StatusOK)
				return
			}
			encodeJSON(t, w, []AppUsage{testAppUsage(false)})
		})
		defer server.Close()

		got, err := client.AttachAppToProject(context.Background(), testAppID, testProjectID)
		if checkErr(t, err, false) {
			return
		}
		if got.ID != testAppUsageID {
			t.Fatalf(fmtUnexpectedID, got.ID, testAppUsageID)
		}
	})

	t.Run("errors when the usage cannot be resolved", func(t *testing.T) {
		t.Parallel()

		client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == httpMethodPost {
				w.WriteHeader(http.StatusOK)
				return
			}
			encodeJSON(t, w, []AppUsage{})
		})
		defer server.Close()

		_, err := client.AttachAppToProject(context.Background(), testAppID, testProjectID)
		checkErr(t, err, true)
	})
}

func TestSetAppUsageEnabled(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != httpMethodPost {
			t.Errorf(fmtUnexpectedMethod, r.Method, httpMethodPost)
		}
		if r.URL.Path != testUsagePath {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		var payload appUsageEnabledPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("failed to decode payload: %v", err)
		}
		if payload.Enabled {
			t.Error("expected enabled=false in payload")
		}
		encodeJSON(t, w, testAppUsage(false))
	})
	defer server.Close()

	got, err := client.SetAppUsageEnabled(context.Background(), testAppID, testAppUsageID, false)
	if checkErr(t, err, false) {
		return
	}
	if got.Enabled {
		t.Fatal("expected usage to be disabled")
	}
}

func TestDeleteAppUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "deletes usage", status: http.StatusOK},
		{name: "treats missing usage as success", status: http.StatusNotFound},
		{name: testCaseServerFailure, status: http.StatusInternalServerError, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodDelete {
					t.Errorf(fmtUnexpectedMethod, r.Method, httpMethodDelete)
				}
				w.WriteHeader(tc.status)
			})
			defer server.Close()

			err := client.DeleteAppUsage(context.Background(), testAppID, testAppUsageID)
			checkErr(t, err, tc.wantErr)
		})
	}
}

func TestDetachAppFromProject(t *testing.T) {
	t.Parallel()

	t.Run("deletes the matching usage", func(t *testing.T) {
		t.Parallel()

		var deletedPath string
		client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == httpMethodDelete {
				deletedPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
				return
			}
			encodeJSON(t, w, []AppUsage{testAppUsage(true)})
		})
		defer server.Close()

		if err := client.DetachAppFromProject(context.Background(), testAppID, testProjectID); err != nil {
			t.Fatalf(fmtUnexpectedError, err)
		}
		if deletedPath != testUsagePath {
			t.Fatalf("unexpected deleted path: %s", deletedPath)
		}
	})

	t.Run("is a no-op when the app is not attached", func(t *testing.T) {
		t.Parallel()

		client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == httpMethodDelete {
				t.Error("unexpected delete request for a project the app is not attached to")
			}
			encodeJSON(t, w, []AppUsage{})
		})
		defer server.Close()

		if err := client.DetachAppFromProject(context.Background(), testAppID, testProjectID); err != nil {
			t.Fatalf(fmtUnexpectedError, err)
		}
	})
}

func TestEnableAppForProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		usages      []AppUsage
		attachedRes AppUsage
		wantPosts   int
	}{
		{
			name:      "already enabled is a no-op",
			usages:    []AppUsage{testAppUsage(true)},
			wantPosts: 0,
		},
		{
			name:      "attached but disabled is toggled",
			usages:    []AppUsage{testAppUsage(false)},
			wantPosts: 1,
		},
		{
			name:        "not attached is attached then enabled",
			usages:      []AppUsage{},
			attachedRes: testAppUsage(false),
			wantPosts:   2,
		},
		{
			name:        "attach that returns an enabled usage skips the toggle",
			usages:      []AppUsage{},
			attachedRes: testAppUsage(true),
			wantPosts:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			posts := 0
			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodPost {
					encodeJSON(t, w, tc.usages)
					return
				}

				posts++
				if strings.HasSuffix(r.URL.Path, appUsagesSubPath) {
					encodeJSON(t, w, tc.attachedRes)
					return
				}
				encodeJSON(t, w, testAppUsage(true))
			})
			defer server.Close()

			got, err := client.EnableAppForProject(context.Background(), testAppID, testProjectID)
			if checkErr(t, err, false) {
				return
			}
			if !got.Enabled {
				t.Fatalf("expected an enabled usage, got %+v", got)
			}
			if posts != tc.wantPosts {
				t.Fatalf("unexpected write count: got %d, want %d", posts, tc.wantPosts)
			}
		})
	}
}

func TestDisableAppForProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		usages    []AppUsage
		wantNil   bool
		wantPosts int
	}{
		{name: "enabled usage is disabled", usages: []AppUsage{testAppUsage(true)}, wantPosts: 1},
		{name: "already disabled is a no-op", usages: []AppUsage{testAppUsage(false)}, wantPosts: 0},
		{name: "not attached returns nil", usages: []AppUsage{}, wantNil: true, wantPosts: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			posts := 0
			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == httpMethodPost {
					posts++
					encodeJSON(t, w, testAppUsage(false))
					return
				}
				encodeJSON(t, w, tc.usages)
			})
			defer server.Close()

			got, err := client.DisableAppForProject(context.Background(), testAppID, testProjectID)
			if checkErr(t, err, false) {
				return
			}
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil usage, got %+v", got)
				}
			} else if got == nil || got.Enabled {
				t.Fatalf("expected a disabled usage, got %+v", got)
			}
			if posts != tc.wantPosts {
				t.Fatalf("unexpected write count: got %d, want %d", posts, tc.wantPosts)
			}
		})
	}
}

func TestEnableAppForAllProjects(t *testing.T) {
	t.Parallel()

	t.Run("enables the app in every project", func(t *testing.T) {
		t.Parallel()

		enabled := map[string]bool{}
		client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == testProjectsAPI:
				encodeJSON(t, w, []Project{{ID: testProjectID}, {ID: "0-3"}})
			case r.Method == httpMethodPost:
				var payload appUsageAttachPayload
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Errorf("failed to decode payload: %v", err)
				}
				enabled[payload.Project.ID] = true
				encodeJSON(t, w, AppUsage{ID: testAppUsageID, Enabled: true, Project: payload.Project})
			default:
				encodeJSON(t, w, []AppUsage{})
			}
		})
		defer server.Close()

		got, err := client.EnableAppForAllProjects(context.Background(), testAppID)
		if checkErr(t, err, false) {
			return
		}
		if len(got) != 2 {
			t.Fatalf("unexpected usages count: got %d, want 2", len(got))
		}
		if !enabled[testProjectID] || !enabled["0-3"] {
			t.Fatalf("expected both projects to be attached, got %v", enabled)
		}
	})

	t.Run("returns partial results on failure", func(t *testing.T) {
		t.Parallel()

		client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == testProjectsAPI:
				encodeJSON(t, w, []Project{{ID: testProjectID}, {ID: "0-3"}})
			case r.Method == httpMethodPost:
				var payload appUsageAttachPayload
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Errorf("failed to decode payload: %v", err)
				}
				if payload.Project.ID == "0-3" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				encodeJSON(t, w, AppUsage{ID: testAppUsageID, Enabled: true, Project: payload.Project})
			default:
				encodeJSON(t, w, []AppUsage{})
			}
		})
		defer server.Close()

		got, err := client.EnableAppForAllProjects(context.Background(), testAppID)
		if err == nil {
			t.Fatal(errExpectedError)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 usage before the failure, got %d", len(got))
		}
	})
}

func TestAppAuthorUnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    AppAuthor
		wantErr bool
	}{
		{name: "plain string", input: `"JetBrains"`, want: "JetBrains"},
		{name: "object with name", input: `{"name":"JetBrains","url":"https://jetbrains.com"}`, want: "JetBrains"},
		{name: "object with login only", input: `{"login":"jb"}`, want: "jb"},
		{name: "null", input: `null`, want: ""},
		{name: "unsupported shape", input: `[1,2]`, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got AppAuthor
			err := json.Unmarshal([]byte(tc.input), &got)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got != tc.want {
				t.Fatalf("unexpected author: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAppAuthorMarshalJSON(t *testing.T) {
	t.Parallel()

	got, err := json.Marshal(App{ID: testAppID, Author: "JetBrains"})
	if checkErr(t, err, false) {
		return
	}
	if !strings.Contains(string(got), `"author":"JetBrains"`) {
		t.Fatalf("unexpected marshaled app: %s", got)
	}
}

func TestListProjects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
		wantLen int
	}{
		{
			name: "returns projects list",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != testProjectsAPI {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}
				if got := r.URL.Query().Get("$skip"); got != "5" {
					t.Errorf("unexpected $skip: got %s, want 5", got)
				}
				encodeJSON(t, w, []Project{{ID: testProjectID, Name: "Demo"}, {ID: "0-3", Name: "Other"}})
			},
			wantLen: 2,
		},
		{
			name: testCaseServerFailure,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, tc.handler)
			defer server.Close()

			got, err := client.ListProjects(context.Background(), 10, 5)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if len(got) != tc.wantLen {
				t.Fatalf("unexpected projects count: got %d, want %d", len(got), tc.wantLen)
			}
		})
	}
}
