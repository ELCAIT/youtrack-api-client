package youtrack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testRoleID         = "role-1"
	testRoleKey        = "developer"
	testRoleName       = "Developer"
	testPermID         = "perm-1"
	testPermKey        = "read-project"
	testPermName       = "Read Project"
	testPermNameA      = "Perm A"
	testPermNameB      = "Perm B"
	testUpdatedName    = "Updated Name"
	testPermIDYT1      = "yt-1"
	testPermIDYT2      = "yt-2"
	testPermIDHub1     = "hub-1"
	testPermIDHub2     = "hub-2"
	testInvalidJSON    = "not json"
	errExpectedError   = "expected error, got nil"
	fmtUnexpectedError = "unexpected error: %v"
	fmtUnexpectedID    = "unexpected id: got %s, want %s"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(handler)
	client, err := NewClient(server.URL, "token")
	if err != nil {
		server.Close()
		t.Fatalf("failed to create client: %v", err)
	}

	return client, server
}

func encodeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()

	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("failed to encode response: %v", err)
	}
}

// checkErr asserts error expectations. Returns true when the caller should stop.
func checkErr(t *testing.T, err error, wantErr bool) bool {
	t.Helper()

	if wantErr {
		if err == nil {
			t.Fatal(errExpectedError)
		}
		return true
	}
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}

	return false
}

// --- mergePermissionLists ---

func TestMergePermissionLists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		primary   []Permission
		secondary []Permission
		wantLen   int
		wantNames []string
	}{
		{
			name:      "primary takes precedence over duplicate names",
			primary:   []Permission{{Id: testPermIDYT1, Name: testPermName}},
			secondary: []Permission{{Id: testPermIDHub1, Name: testPermName}},
			wantLen:   1,
			wantNames: []string{testPermName},
		},
		{
			name:      "secondary appended when not in primary",
			primary:   []Permission{{Id: testPermIDYT1, Name: testPermNameA}},
			secondary: []Permission{{Id: testPermIDHub1, Name: testPermNameB}},
			wantLen:   2,
			wantNames: []string{testPermNameA, testPermNameB},
		},
		{
			name:      "name comparison is case-insensitive",
			primary:   []Permission{{Id: testPermIDYT1, Name: "PERM A"}},
			secondary: []Permission{{Id: testPermIDHub1, Name: "perm a"}},
			wantLen:   1,
			wantNames: []string{"PERM A"},
		},
		{
			name:      "empty primary returns secondary",
			primary:   []Permission{},
			secondary: []Permission{{Id: testPermIDHub1, Name: testPermNameB}},
			wantLen:   1,
			wantNames: []string{testPermNameB},
		},
		{
			name:      "empty secondary returns primary",
			primary:   []Permission{{Id: testPermIDYT1, Name: testPermNameA}},
			secondary: []Permission{},
			wantLen:   1,
			wantNames: []string{testPermNameA},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := mergePermissionLists(tc.primary, tc.secondary)
			if len(got) != tc.wantLen {
				t.Fatalf("got %d permissions, want %d", len(got), tc.wantLen)
			}
			for i, name := range tc.wantNames {
				if got[i].Name != name {
					t.Errorf("got[%d].Name = %q, want %q", i, got[i].Name, name)
				}
			}
		})
	}
}

// newPermissionsDispatchHandler routes requests to ytHandler when the path
// contains the YouTrack permissions API path, and to hubHandler otherwise.
func newPermissionsDispatchHandler(ytHandler, hubHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, youtrackPermissionsAPIPath) {
			ytHandler(w, r)
			return
		}
		hubHandler(w, r)
	}
}

// --- GetAllPermissions ---

func TestGetAllPermissions(t *testing.T) {
	t.Parallel()

	ytPerm := Permission{Id: testPermIDYT1, Key: "yt.read", Name: "YT Read"}
	hubPerm := Permission{Id: testPermIDHub1, Key: "hub.write", Name: "Hub Write"}
	sharedPerm := Permission{Id: testPermIDYT2, Key: "yt.shared", Name: "Shared"}
	hubDupePerm := Permission{Id: testPermIDHub2, Key: "hub.shared", Name: "Shared"} // same name as sharedPerm

	serverError := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }
	encodeYTPerms := func(w http.ResponseWriter, _ *http.Request) {
		encodeJSON(t, w, []Permission{ytPerm, sharedPerm})
	}
	encodeHubPerms := func(w http.ResponseWriter, _ *http.Request) {
		encodeJSON(t, w, PermissionsResponse{Permissions: []Permission{hubPerm, hubDupePerm}})
	}
	encodeYTPerm := func(w http.ResponseWriter, _ *http.Request) { encodeJSON(t, w, []Permission{ytPerm}) }
	encodeHubPerm := func(w http.ResponseWriter, _ *http.Request) {
		encodeJSON(t, w, PermissionsResponse{Permissions: []Permission{hubPerm}})
	}

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantLen int
		wantErr bool
	}{
		{
			name:    "merges youtrack and hub permissions, deduplicates by name",
			handler: newPermissionsDispatchHandler(encodeYTPerms, encodeHubPerms),
			wantLen: 3,
		},
		{
			name:    "error on hub permissions request",
			handler: newPermissionsDispatchHandler(encodeYTPerm, serverError),
			wantErr: true,
		},
		{
			name:    "error on youtrack permissions request",
			handler: newPermissionsDispatchHandler(serverError, encodeHubPerm),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, tc.handler)
			defer server.Close()

			got, err := client.GetAllPermissions(context.Background())
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if len(got) != tc.wantLen {
				t.Fatalf("got %d permissions, want %d", len(got), tc.wantLen)
			}
		})
	}
}

// --- GetPermissionGraph ---

func TestGetPermissionGraph(t *testing.T) {
	t.Parallel()

	graphEntry := PermissionGraphEntry{
		Id:   testPermIDYT1,
		Key:  "JetBrains.YouTrack.READ_ISSUE",
		Name: "Read Issue",
		ImpliedPermissions: []PermissionGraphEntry{
			{Id: testPermIDYT2, Key: "jetbrains.jetpass.project-basic-read", Name: "Read Project Basic"},
		},
	}

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantLen int
		wantErr bool
	}{
		{
			name: "returns permission graph",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				encodeJSON(t, w, []PermissionGraphEntry{graphEntry})
			},
			wantLen: 1,
		},
		{
			name: "returns error on non-2xx",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
		{
			name: "returns error on invalid JSON",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(testInvalidJSON))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, tc.handler)
			defer server.Close()

			got, err := client.GetPermissionGraph(context.Background())
			if checkErr(t, err, tc.wantErr) {
				return
			}

			if len(got) != tc.wantLen {
				t.Fatalf("got %d permissions, want %d", len(got), tc.wantLen)
			}
		})
	}
}

// --- GetYoutrackRoleById ---

func TestGetYoutrackRoleById(t *testing.T) {
	t.Parallel()

	role := Role{Id: testRoleID, Key: testRoleKey, Name: testRoleName, Permissions: []Permission{{Id: testPermID, Key: testPermKey, Name: testPermName}}}

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantID  string
		wantErr bool
	}{
		{
			name: "returns role by id",
			handler: func(w http.ResponseWriter, r *http.Request) {
				encodeJSON(t, w, role)
			},
			wantID: testRoleID,
		},
		{
			name: "returns error on 404",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: true,
		},
		{
			name: "returns error on invalid JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(testInvalidJSON))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, tc.handler)
			defer server.Close()

			got, err := client.GetYoutrackRoleById(context.Background(), testRoleID)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.Id != tc.wantID {
				t.Fatalf(fmtUnexpectedID, got.Id, tc.wantID)
			}
		})
	}
}

// --- CreateYoutrackRole ---

func TestCreateYoutrackRole(t *testing.T) {
	t.Parallel()

	created := Role{Id: testRoleID, Key: testRoleKey, Name: testRoleName}

	encodeCreated := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf(errUnexpectedMethod, r.Method)
		}
		encodeJSON(t, w, created)
	}
	serverError := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }
	writeInvalidJSON := func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(testInvalidJSON)) }

	tests := []struct {
		name    string
		input   Role
		handler http.HandlerFunc
		wantID  string
		wantErr bool
	}{
		{
			name:    "creates role and returns it",
			input:   Role{Key: testRoleKey, Name: testRoleName},
			handler: encodeCreated,
			wantID:  testRoleID,
		},
		{
			name:    "returns error on server failure",
			input:   Role{Key: testRoleKey, Name: testRoleName},
			handler: serverError,
			wantErr: true,
		},
		{
			name:    "returns error on invalid JSON response",
			input:   Role{Key: testRoleKey, Name: testRoleName},
			handler: writeInvalidJSON,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, tc.handler)
			defer server.Close()

			got, err := client.CreateYoutrackRole(context.Background(), tc.input)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.Id != tc.wantID {
				t.Fatalf(fmtUnexpectedID, got.Id, tc.wantID)
			}
		})
	}
}

// --- UpdateYoutrackRole ---

func TestUpdateYoutrackRole(t *testing.T) {
	t.Parallel()

	updated := Role{Id: testRoleID, Key: testRoleKey, Name: testUpdatedName}

	tests := []struct {
		name     string
		input    Role
		handler  http.HandlerFunc
		wantName string
		wantErr  bool
	}{
		{
			name:  "updates role and returns refreshed state",
			input: Role{Id: testRoleID, Name: testUpdatedName},
			handler: func(w http.ResponseWriter, r *http.Request) {
				// Both POST (update) and GET (refresh) return the updated role
				encodeJSON(t, w, updated)
			},
			wantName: testUpdatedName,
		},
		{
			name:  "returns error when update POST fails",
			input: Role{Id: testRoleID, Name: testUpdatedName},
			handler: func(w http.ResponseWriter, r *http.Request) {
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

			got, err := client.UpdateYoutrackRole(context.Background(), tc.input)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.Name != tc.wantName {
				t.Fatalf("got name %q, want %q", got.Name, tc.wantName)
			}
		})
	}
}

// --- DeleteYoutrackRole ---

func TestDeleteYoutrackRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "deletes role successfully",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("unexpected method: %s", r.Method)
				}
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "idempotent on 404",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
		},
		{
			name: "returns error on server failure",
			handler: func(w http.ResponseWriter, r *http.Request) {
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

			err := client.DeleteYoutrackRole(context.Background(), testRoleID)
			if checkErr(t, err, tc.wantErr) {
				return
			}
		})
	}
}

// --- ListYoutrackRoles / GetYoutrackRoleByName ---

func TestListYoutrackRoles(t *testing.T) {
	t.Parallel()

	roles := []Role{
		{Id: testRoleID, Key: testRoleKey, Name: testRoleName},
		{Id: "role-2", Key: "reader", Name: "ELCA Reader"},
	}

	tests := []struct {
		name      string
		top, skip int
		handler   http.HandlerFunc
		wantLen   int
		wantErr   bool
	}{
		{
			name: "returns roles",
			handler: func(w http.ResponseWriter, r *http.Request) {
				encodeJSON(t, w, roles)
			},
			wantLen: 2,
		},
		{
			name: "passes pagination to the query",
			top:  10,
			skip: 20,
			handler: func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				if q.Get("$top") != "10" || q.Get("$skip") != "20" {
					t.Errorf("unexpected pagination: $top=%q $skip=%q", q.Get("$top"), q.Get("$skip"))
				}
				if !strings.Contains(q.Get("fields"), "permissions(") {
					t.Errorf("unexpected fields: %q", q.Get("fields"))
				}
				encodeJSON(t, w, roles)
			},
			wantLen: 2,
		},
		{
			name: "returns error on server failure",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
		{
			name: "returns error on invalid JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(testInvalidJSON))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, tc.handler)
			defer server.Close()

			got, err := client.ListYoutrackRoles(context.Background(), tc.top, tc.skip)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if len(got) != tc.wantLen {
				t.Fatalf("unexpected role count: got %d, want %d", len(got), tc.wantLen)
			}
		})
	}
}

func TestGetYoutrackRoleByName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		lookup   string
		handler  http.HandlerFunc
		wantID   string
		wantErr  bool
		wantMiss bool
	}{
		{
			name:   "finds an exact match",
			lookup: "ELCA Reader",
			handler: func(w http.ResponseWriter, r *http.Request) {
				encodeJSON(t, w, []Role{{Id: testRoleID, Name: testRoleName}, {Id: "role-2", Name: "ELCA Reader"}})
			},
			wantID: "role-2",
		},
		{
			name:   "falls back to a case-insensitive match",
			lookup: "elca reader",
			handler: func(w http.ResponseWriter, r *http.Request) {
				encodeJSON(t, w, []Role{{Id: "role-2", Name: "ELCA Reader"}})
			},
			wantID: "role-2",
		},
		{
			name:   "reports not found when no role matches",
			lookup: "ELCA Nonexistent",
			handler: func(w http.ResponseWriter, r *http.Request) {
				encodeJSON(t, w, []Role{{Id: testRoleID, Name: testRoleName}})
			},
			wantErr:  true,
			wantMiss: true,
		},
		{
			name:   "reports not found on an empty instance",
			lookup: "ELCA Reader",
			handler: func(w http.ResponseWriter, r *http.Request) {
				encodeJSON(t, w, []Role{})
			},
			wantErr:  true,
			wantMiss: true,
		},
		{
			// A transport failure must never be reported as absence: a caller that
			// treats it as "role missing" would fail startup validation spuriously.
			name:   "does not report not found on server failure",
			lookup: "ELCA Reader",
			handler: func(w http.ResponseWriter, r *http.Request) {
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

			got, err := client.GetYoutrackRoleByName(context.Background(), tc.lookup)
			if tc.wantErr {
				assertRoleLookupFailed(t, err, tc.wantMiss)
				return
			}
			assertRoleFound(t, got, err, tc.wantID)
		})
	}
}

// assertRoleLookupFailed checks that a failed lookup reports absence exactly when
// wantMiss says it should. Both predicates must agree: a miss satisfies the general
// and the role-specific one, while any other failure satisfies neither.
func assertRoleLookupFailed(t *testing.T, err error, wantMiss bool) {
	t.Helper()

	if err == nil {
		t.Fatal(errExpectedError)
	}
	if IsNotFound(err) != wantMiss {
		t.Fatalf("IsNotFound = %v, want %v (err: %v)", IsNotFound(err), wantMiss, err)
	}
	if IsRoleNotFoundError(err) != wantMiss {
		t.Fatalf("IsRoleNotFoundError = %v, want %v (err: %v)", IsRoleNotFoundError(err), wantMiss, err)
	}
}

// assertRoleFound checks that a successful lookup returned the expected role.
func assertRoleFound(t *testing.T, got *Role, err error, wantID string) {
	t.Helper()

	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if got.Id != wantID {
		t.Fatalf(fmtUnexpectedID, got.Id, wantID)
	}
}

// TestGetYoutrackRoleByNamePagesUntilShortPage proves the lookup does not stop at the
// first page: the match only appears on page two.
func TestGetYoutrackRoleByNamePagesUntilShortPage(t *testing.T) {
	t.Parallel()

	firstPage := make([]Role, roleLookupPageSize)
	for i := range firstPage {
		firstPage[i] = Role{Id: "filler", Name: "Filler Role"}
	}

	var pages int
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		pages++
		if r.URL.Query().Get("$skip") == "" {
			encodeJSON(t, w, firstPage)
			return
		}
		encodeJSON(t, w, []Role{{Id: "role-2", Name: "ELCA Reader"}})
	})
	defer server.Close()

	got, err := client.GetYoutrackRoleByName(context.Background(), "ELCA Reader")
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if got.Id != "role-2" {
		t.Fatalf(fmtUnexpectedID, got.Id, "role-2")
	}
	if pages != 2 {
		t.Fatalf("unexpected page count: got %d, want 2", pages)
	}
}

// TestRoleNotFoundPredicateDoesNotCrossTalk keeps the entity-specific predicate
// narrow: an app miss must not read as a role miss, and vice versa.
func TestRoleNotFoundPredicateDoesNotCrossTalk(t *testing.T) {
	t.Parallel()

	roleMiss := entityNotFoundf(errRoleNotFound, "role with name %q", "ELCA Reader")
	appMiss := entityNotFoundf(errAppNotFound, "app with name %q", "Diagram Editor")

	if !IsRoleNotFoundError(roleMiss) || !IsNotFound(roleMiss) {
		t.Fatal("role miss should satisfy both IsRoleNotFoundError and IsNotFound")
	}
	if IsRoleNotFoundError(appMiss) {
		t.Fatal("app miss must not satisfy IsRoleNotFoundError")
	}
	if IsAppNotFoundError(roleMiss) {
		t.Fatal("role miss must not satisfy IsAppNotFoundError")
	}
}
