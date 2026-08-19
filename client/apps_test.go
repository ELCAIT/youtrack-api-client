package youtrack

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const (
	testAppID          = "145-92"
	testAppName        = "Test App"
	testAppUsageID     = "usage-1"
	testAppsPath       = "/api/admin/apps"
	testAppPath        = "/api/admin/apps/145-92"
	testUsagesPath     = "/api/admin/apps/145-92/usages"
	testUsagePath      = "/api/admin/apps/145-92/usages/usage-1"
	testProjectsAPI    = "/api/admin/projects"
	testOtherProjectID = "0-3"

	fmtUnexpectedPath   = "unexpected path: %s"
	fmtUnexpectedMethod = "unexpected method: got %s, want %s"
)

func testAppUsage(enabled bool) AppUsage {
	return AppUsage{ID: testAppUsageID, Enabled: enabled, Project: &Project{ID: testProjectID}}
}

// appAPIStub describes how the fake apps API answers each request shape, so the
// activation tests can be table-driven instead of each carrying its own
// handler closure. A zero-value attachRes makes an attach answer with an empty
// body, which is what exercises the re-fetch path in AttachAppToProject.
type appAPIStub struct {
	projects       []Project  // answer for a GET on the projects endpoint
	usages         []AppUsage // answer for a GET on the usages collection
	attachRes      AppUsage   // answer for a POST on the usages collection
	toggleRes      AppUsage   // answer for a POST on a single usage
	attachFailsFor string     // project ID whose attach answers 403 instead
	posts          *int       // incremented on every POST, when set
}

// newAppAPIHandler builds the fake apps API described by stub.
func newAppAPIHandler(t *testing.T, stub *appAPIStub) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == testProjectsAPI:
			encodeJSON(t, w, stub.projects)
		case r.Method == httpMethodPost:
			stub.servePost(t, w, r)
		default:
			encodeJSON(t, w, stub.usages)
		}
	}
}

// servePost answers the two POST shapes the client uses: an attach on the
// usages collection, and an enable/disable toggle on a single usage.
func (s *appAPIStub) servePost(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	if s.posts != nil {
		*s.posts++
	}

	if !strings.HasSuffix(r.URL.Path, appUsagesSubPath) {
		encodeJSON(t, w, s.toggleRes)
		return
	}

	project := decodeAttachPayload(t, r)
	if project.ID == s.attachFailsFor {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if s.attachRes.ID == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Echo the posted project back, so asserting on the returned usage also
	// proves the request body carried the right project.
	created := s.attachRes
	created.Project = project
	encodeJSON(t, w, created)
}

// decodeAttachPayload reads the project reference out of an attach request.
func decodeAttachPayload(t *testing.T, r *http.Request) *Project {
	t.Helper()

	var payload appUsageAttachPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode attach payload: %v", err)
	}
	if payload.Project == nil {
		t.Fatal("attach request carried no project")
	}

	return payload.Project
}

// assertUsageEnabled asserts a usage exists, points at the expected project,
// and is in the expected enabled state.
func assertUsageEnabled(t *testing.T, usage *AppUsage, projectID string, wantEnabled bool) {
	t.Helper()

	if usage == nil {
		t.Fatal("expected an app usage, got nil")
	}
	if usage.ID == "" {
		t.Fatal("app usage has an empty id")
	}
	if usage.Project == nil || usage.Project.ID != projectID {
		t.Fatalf("app usage points at the wrong project: %+v", usage.Project)
	}
	if usage.Enabled != wantEnabled {
		t.Fatalf("unexpected enabled state: got %t, want %t", usage.Enabled, wantEnabled)
	}
}

// assertNoUsage asserts the app is not attached to the project.
func assertNoUsage(t *testing.T, usage *AppUsage) {
	t.Helper()

	if usage != nil {
		t.Fatalf("expected nil usage, got %+v", usage)
	}
}

// assertPostCount asserts how many write requests the client issued.
func assertPostCount(t *testing.T, got, want int) {
	t.Helper()

	if got != want {
		t.Fatalf("unexpected write count: got %d, want %d", got, want)
	}
}

// assertUsagesForProjects asserts that usages cover exactly wantProjectIDs, in
// order, and that each one is enabled.
func assertUsagesForProjects(t *testing.T, usages []AppUsage, wantProjectIDs []string) {
	t.Helper()

	if len(usages) != len(wantProjectIDs) {
		t.Fatalf("unexpected usages count: got %d, want %d", len(usages), len(wantProjectIDs))
	}
	for i, wantProjectID := range wantProjectIDs {
		assertUsageEnabled(t, &usages[i], wantProjectID, true)
	}
}

// checkPartialErr asserts the error expectation without the early return
// checkErr does, so the caller can still inspect a partial result.
func checkPartialErr(t *testing.T, err error, wantErr bool) {
	t.Helper()

	if wantErr && err == nil {
		t.Fatal(errExpectedError)
	}
	if !wantErr && err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
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
					{ID: testAppID, Name: testAppName, Title: "Test"},
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
		encodeJSON(t, w, App{
			ID:      testAppID,
			Name:    testAppName,
			Title:   "Test",
			Version: "1.0.0",
			Vendor:  &AppVendor{Name: "JetBrains s.r.o.", URL: "https://jetbrains.com"},
		})
	})
	defer server.Close()

	got, err := client.GetAppByID(context.Background(), testAppID)
	if checkErr(t, err, false) {
		return
	}
	if got.ID != testAppID {
		t.Fatalf(fmtUnexpectedID, got.ID, testAppID)
	}
	if got.Vendor == nil || got.Vendor.Name != "JetBrains s.r.o." {
		t.Fatalf("unexpected vendor: %+v", got.Vendor)
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

	tests := []struct {
		name        string
		stub        appAPIStub
		wantErr     bool
		wantEnabled bool
	}{
		{
			name:        "returns created usage",
			stub:        appAPIStub{attachRes: testAppUsage(true)},
			wantEnabled: true,
		},
		{
			// An empty attach response makes the client fall back to looking
			// the usage up in the usages collection.
			name: "refetches usage on empty response body",
			stub: appAPIStub{usages: []AppUsage{testAppUsage(false)}},
		},
		{
			name:    "errors when the usage cannot be resolved",
			stub:    appAPIStub{usages: []AppUsage{}},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, newAppAPIHandler(t, &tc.stub))
			defer server.Close()

			got, err := client.AttachAppToProject(context.Background(), testAppID, testProjectID)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			assertUsageEnabled(t, got, testProjectID, tc.wantEnabled)
		})
	}
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
		name      string
		stub      appAPIStub
		wantPosts int
	}{
		{
			name:      "already enabled is a no-op",
			stub:      appAPIStub{usages: []AppUsage{testAppUsage(true)}},
			wantPosts: 0,
		},
		{
			name:      "attached but disabled is toggled",
			stub:      appAPIStub{usages: []AppUsage{testAppUsage(false)}},
			wantPosts: 1,
		},
		{
			name:      "not attached is attached then enabled",
			stub:      appAPIStub{usages: []AppUsage{}, attachRes: testAppUsage(false)},
			wantPosts: 2,
		},
		{
			name:      "attach that returns an enabled usage skips the toggle",
			stub:      appAPIStub{usages: []AppUsage{}, attachRes: testAppUsage(true)},
			wantPosts: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			posts := 0
			stub := tc.stub
			stub.posts = &posts
			stub.toggleRes = testAppUsage(true)

			client, server := newTestClient(t, newAppAPIHandler(t, &stub))
			defer server.Close()

			got, err := client.EnableAppForProject(context.Background(), testAppID, testProjectID)
			if checkErr(t, err, false) {
				return
			}
			assertUsageEnabled(t, got, testProjectID, true)
			assertPostCount(t, posts, tc.wantPosts)
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
			client, server := newTestClient(t, newAppAPIHandler(t, &appAPIStub{
				usages:    tc.usages,
				toggleRes: testAppUsage(false),
				posts:     &posts,
			}))
			defer server.Close()

			got, err := client.DisableAppForProject(context.Background(), testAppID, testProjectID)
			if checkErr(t, err, false) {
				return
			}
			if tc.wantNil {
				assertNoUsage(t, got)
			} else {
				assertUsageEnabled(t, got, testProjectID, false)
			}
			assertPostCount(t, posts, tc.wantPosts)
		})
	}
}

func TestEnableAppForAllProjects(t *testing.T) {
	t.Parallel()

	projects := []Project{{ID: testProjectID}, {ID: testOtherProjectID}}

	tests := []struct {
		name           string
		attachFailsFor string
		wantErr        bool
		wantProjects   []string
	}{
		{
			name:         "enables the app in every project",
			wantProjects: []string{testProjectID, testOtherProjectID},
		},
		{
			// The second project fails, so the first one's usage must still
			// come back alongside the error.
			name:           "returns partial results on failure",
			attachFailsFor: testOtherProjectID,
			wantErr:        true,
			wantProjects:   []string{testProjectID},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, newAppAPIHandler(t, &appAPIStub{
				projects:       projects,
				usages:         []AppUsage{},
				attachRes:      AppUsage{ID: testAppUsageID, Enabled: true},
				attachFailsFor: tc.attachFailsFor,
			}))
			defer server.Close()

			got, err := client.EnableAppForAllProjects(context.Background(), testAppID)
			checkPartialErr(t, err, tc.wantErr)
			assertUsagesForProjects(t, got, tc.wantProjects)
		})
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
				encodeJSON(t, w, []Project{{ID: testProjectID, Name: "Demo"}, {ID: testOtherProjectID, Name: "Other"}})
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
