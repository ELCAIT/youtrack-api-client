package youtrack

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	testGroupID         = "group-1"
	testGroupName       = "Dev Team"
	testUserLogin       = "alice"
	testUserID          = "user-1"
	testOtherUserID     = "user-2"
	testOtherUserLogin  = "bob"
	testUserFullName    = "Alice Doe"
	testUserEmail       = "alice@example.com"
	testAllUsersGroupID = "group-all"
	testDevelopersGroup = "Developers"
)

// --- GetUserByLogin ---

func TestGetUserByLogin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		responseBody any
		lookupLogin  string
		wantID       string
		wantErr      bool
	}{
		{
			name:         "plain array format",
			responseBody: []Holder{{Id: testUserID, Login: testUserLogin}, {Id: testOtherUserID, Login: testOtherUserLogin}},
			lookupLogin:  testUserLogin,
			wantID:       testUserID,
		},
		{
			name:         "wrapped users format",
			responseBody: map[string]any{"users": []Holder{{Id: testUserID, Login: testUserLogin}}},
			lookupLogin:  testUserLogin,
			wantID:       testUserID,
		},
		{
			name:         "not found",
			responseBody: []Holder{{Id: testOtherUserID, Login: testOtherUserLogin}},
			lookupLogin:  testUserLogin,
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				encodeJSON(t, w, tc.responseBody)
			})
			defer server.Close()

			got, err := client.GetUserByLogin(context.Background(), tc.lookupLogin)
			if tc.wantErr {
				if err == nil {
					t.Fatal(errExpectedError)
				}
				return
			}
			if err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
			if got.Id != tc.wantID {
				t.Fatalf(fmtUnexpectedID, got.Id, tc.wantID)
			}
		})
	}
}

// --- GetUserGroupByName ---

func TestGetUserGroupByName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		responseBody any
		lookupName   string
		wantID       string
		wantErr      bool
	}{
		{
			name:         "case-insensitive match",
			responseBody: []Holder{{Id: testGroupID, Name: "DEV TEAM"}},
			lookupName:   "dev team",
			wantID:       testGroupID,
		},
		{
			name:         "wrapped usergroups format",
			responseBody: map[string]any{"usergroups": []Holder{{Id: testGroupID, Name: testGroupName}}},
			lookupName:   testGroupName,
			wantID:       testGroupID,
		},
		{
			name:         "not found",
			responseBody: []Holder{{Id: "group-2", Name: "Other Team"}},
			lookupName:   testGroupName,
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				encodeJSON(t, w, tc.responseBody)
			})
			defer server.Close()

			got, err := client.GetUserGroupByName(context.Background(), tc.lookupName)
			if tc.wantErr {
				if err == nil {
					t.Fatal(errExpectedError)
				}
				return
			}
			if err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
			if got.Id != tc.wantID {
				t.Fatalf(fmtUnexpectedID, got.Id, tc.wantID)
			}
		})
	}
}

// --- GetAllUsersGroup ---

func TestGetAllUsersGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		groups  []NestedGroup
		wantID  string
		wantErr bool
	}{
		{
			name: "returns the all-users group",
			groups: []NestedGroup{
				{ID: "group-regular", Name: testDevelopersGroup, AllUsersGroup: false},
				{ID: testAllUsersGroupID, Name: "All Users", AllUsersGroup: true},
			},
			wantID: testAllUsersGroupID,
		},
		{
			name:    "errors when no all-users group present",
			groups:  []NestedGroup{{ID: testGroupID, Name: testDevelopersGroup, AllUsersGroup: false}},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				encodeJSON(t, w, tc.groups)
			})
			defer server.Close()

			got, err := client.GetAllUsersGroup(context.Background())
			if tc.wantErr {
				if err == nil {
					t.Fatal(errExpectedError)
				}
				return
			}
			if err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
			if got.ID != tc.wantID {
				t.Fatalf(fmtUnexpectedID, got.ID, tc.wantID)
			}
		})
	}
}

// --- DeleteGroup ---

func newDeleteGroupHandler(t *testing.T, statusCode int, deleteCalled *bool) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf(errUnexpectedMethod, r.Method)
		}
		if r.URL.Query().Get("successor") == "" {
			t.Error("expected successor query parameter")
		}
		*deleteCalled = true
		w.WriteHeader(statusCode)
	}
}

func TestDeleteGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		statusCode     int
		wantDeleteCall bool
		wantErr        bool
	}{
		{
			name:           "success sends DELETE with successor param",
			statusCode:     http.StatusOK,
			wantDeleteCall: true,
			wantErr:        false,
		},
		{
			name:           "404 returns error when no endpoint variant accepts delete",
			statusCode:     http.StatusNotFound,
			wantDeleteCall: true,
			wantErr:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deleteCalled := false

			client, server := newTestClient(t, newDeleteGroupHandler(t, tc.statusCode, &deleteCalled))
			defer server.Close()

			err := client.DeleteGroup(context.Background(), testGroupID, testAllUsersGroupID)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
			if tc.wantDeleteCall && !deleteCalled {
				t.Fatal("expected DELETE request to be called")
			}
		})
	}
}

// --- ListUsers ---

func TestListUsers(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		query := r.URL.Query()
		if query.Get("$top") != "25" {
			t.Fatalf("unexpected $top: got %q, want %q", query.Get("$top"), "25")
		}
		if query.Get("$skip") != "50" {
			t.Fatalf("unexpected $skip: got %q, want %q", query.Get("$skip"), "50")
		}
		if !strings.Contains(query.Get("fields"), "login") {
			t.Fatalf("unexpected fields: %q", query.Get("fields"))
		}

		encodeJSON(t, w, []Holder{{Id: testUserID, Login: testUserLogin}})
	})
	defer server.Close()

	users, err := client.ListUsers(context.Background(), 25, 50)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if len(users) != 1 {
		t.Fatalf("unexpected number of users: got %d, want 1", len(users))
	}
	if users[0].Id != testUserID {
		t.Fatalf(fmtUnexpectedID, users[0].Id, testUserID)
	}
}

// --- ListGroups ---

func TestListGroups(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		query := r.URL.Query()
		if query.Get("$top") != "10" {
			t.Fatalf("unexpected $top: got %q, want %q", query.Get("$top"), "10")
		}
		if query.Get("$skip") != "" {
			t.Fatalf("unexpected $skip: got %q, want empty", query.Get("$skip"))
		}

		encodeJSON(t, w, []Holder{{Id: testGroupID, Name: testGroupName}})
	})
	defer server.Close()

	groups, err := client.ListGroups(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if len(groups) != 1 {
		t.Fatalf("unexpected number of groups: got %d, want 1", len(groups))
	}
	if groups[0].Id != testGroupID {
		t.Fatalf(fmtUnexpectedID, groups[0].Id, testGroupID)
	}
}

// --- CreateUser ---

func TestCreateUser(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if !strings.Contains(string(body), testUserLogin) {
			t.Fatalf("request body does not contain login %q: %s", testUserLogin, string(body))
		}

		encodeJSON(t, w, User{ID: testUserID, Login: testUserLogin, FullName: testUserFullName, Email: testUserEmail})
	})
	defer server.Close()

	created, err := client.CreateUser(context.Background(), User{Login: testUserLogin, FullName: testUserFullName, Email: testUserEmail})
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if created.ID != testUserID {
		t.Fatalf(fmtUnexpectedID, created.ID, testUserID)
	}
}

// --- GetUserByID ---

func TestGetUserByID(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/api/users/"+testUserID) {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if fields := r.URL.Query().Get("fields"); !strings.Contains(fields, "email") || !strings.Contains(fields, "banned") {
			t.Fatalf("fields param does not request email and banned: %q", fields)
		}

		encodeJSON(t, w, User{ID: testUserID, Login: testUserLogin, FullName: testUserFullName, Email: testUserEmail, Banned: true})
	})
	defer server.Close()

	user, err := client.GetUserByID(context.Background(), testUserID)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	want := User{ID: testUserID, Login: testUserLogin, FullName: testUserFullName, Email: testUserEmail, Banned: true}
	if *user != want {
		t.Fatalf("unexpected user: got %+v, want %+v", *user, want)
	}
}

func TestGetUserByIDNotFound(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	_, err := client.GetUserByID(context.Background(), "missing-user")
	if !IsNotFoundError(err) {
		t.Fatalf("IsNotFoundError = false for a 404; got error: %v", err)
	}
}

// --- UpdateUser ---

func TestUpdateUser(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/api/users/"+testUserID) {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		encodeJSON(t, w, User{ID: testUserID, Login: testUserLogin, FullName: "Alice Updated", Email: testUserEmail})
	})
	defer server.Close()

	updated, err := client.UpdateUser(context.Background(), testUserID, User{FullName: "Alice Updated"})
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if updated.FullName != "Alice Updated" {
		t.Fatalf("unexpected fullName: got %q, want %q", updated.FullName, "Alice Updated")
	}
}

func TestBanUser(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/api/users/"+testUserID) {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if !strings.Contains(string(body), `"banned":true`) {
			t.Fatalf("expected banned=true in request body, got %s", string(body))
		}

		encodeJSON(t, w, User{ID: testUserID, Login: testUserLogin, Banned: true})
	})
	defer server.Close()

	updated, err := client.BanUser(context.Background(), testUserID)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if !updated.Banned {
		t.Fatal("expected banned user response")
	}
}

// newSetBannedHandler asserts the request writes exactly the wanted banned value and
// answers with a user carrying it. wantBody is matched verbatim so that an unban, whose
// whole point is that "banned":false survives marshalling, cannot pass on a body that
// omitted the field.
func newSetBannedHandler(t *testing.T, wantBody string, banned bool) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/api/users/"+testUserID) {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if !strings.Contains(string(body), wantBody) {
			t.Fatalf("expected %s in request body, got %s", wantBody, string(body))
		}

		encodeJSON(t, w, User{ID: testUserID, Login: testUserLogin, Banned: banned})
	}
}

func TestUnbanUser(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, newSetBannedHandler(t, `"banned":false`, false))
	defer server.Close()

	updated, err := client.UnbanUser(context.Background(), testUserID)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if updated.Banned {
		t.Fatal("expected an unbanned user response")
	}
}

func TestSetUserBanned(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		banned   bool
		wantBody string
	}{
		{name: "ban", banned: true, wantBody: `"banned":true`},
		{name: "unban", banned: false, wantBody: `"banned":false`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, newSetBannedHandler(t, tc.wantBody, tc.banned))
			defer server.Close()

			updated, err := client.SetUserBanned(context.Background(), testUserID, tc.banned)
			if err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
			if updated.Banned != tc.banned {
				t.Fatalf("banned = %t, want %t", updated.Banned, tc.banned)
			}
		})
	}
}

// --- DeleteUser ---

func newDeleteUserHandler(t *testing.T, statusCode int, validateDeleteRequest bool, readsBeforeGone int32) http.HandlerFunc {
	t.Helper()

	var reads atomic.Int32

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			serveDeleteUserRead(t, w, r, func() bool { return reads.Add(1) <= readsBeforeGone })
			return
		}

		if r.Method != http.MethodDelete {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}

		if validateDeleteRequest {
			assertDeleteUserRequest(t, r)
		}

		w.WriteHeader(statusCode)
	}
}

// serveDeleteUserRead answers the guest successor lookup, and the read-back of
// the deleted user with the user while stillPresent reports true, then with 404.
func serveDeleteUserRead(t *testing.T, w http.ResponseWriter, r *http.Request, stillPresent func() bool) {
	t.Helper()

	switch {
	case !strings.HasSuffix(r.URL.Path, "/api/users/"+testUserID):
		encodeJSON(t, w, []Holder{{Id: "guest-id", Login: "guest"}})
	case stillPresent():
		encodeJSON(t, w, User{ID: testUserID})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func assertDeleteUserRequest(t *testing.T, r *http.Request) {
	t.Helper()

	if !strings.HasSuffix(r.URL.Path, "/api/users/"+testUserID) {
		t.Fatalf("unexpected path: %s", r.URL.Path)
	}
	if r.URL.Query().Get("successor") == "" {
		t.Fatal("expected successor query parameter")
	}
}

func TestDeleteUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		statusCode            int
		validateDeleteRequest bool
		readsBeforeGone       int32
	}{
		{
			name:                  "success",
			statusCode:            http.StatusNoContent,
			validateDeleteRequest: true,
		},
		{
			name:            "waits until hub has processed the delete",
			statusCode:      http.StatusNoContent,
			readsBeforeGone: 2,
		},
		{
			name:       "404 is ignored",
			statusCode: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, newDeleteUserHandler(t, tc.statusCode, tc.validateDeleteRequest, tc.readsBeforeGone))
			defer server.Close()

			if err := client.DeleteUser(context.Background(), testUserID); err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
		})
	}
}

// --- AddUserToGroup ---

func TestAddUserToGroup(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/api/usergroups/"+testGroupID+"/users") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if !strings.Contains(string(body), testUserID) {
			t.Fatalf("request body does not contain user id %q: %s", testUserID, string(body))
		}

		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	if err := client.AddUserToGroup(context.Background(), testGroupID, testUserID); err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
}

func TestAddUserToGroupFallbackOnMethodNotAllowed(t *testing.T) {
	t.Parallel()

	var calls []string

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)

		// Simulate staging behavior where /api/groups membership add is not allowed.
		if strings.HasPrefix(r.URL.Path, "/api/groups/") {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte("Code,405\nError,Method Not Allowed"))
			return
		}

		// Accept hub user-group style endpoint as fallback success.
		if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/api/users/"+testUserID+"/groups/"+testGroupID) {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Code,405\nError,Method Not Allowed"))
	})
	defer server.Close()

	if err := client.AddUserToGroup(context.Background(), testGroupID, testUserID); err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}

	if len(calls) < 2 {
		t.Fatalf("expected multiple fallback calls, got %d", len(calls))
	}
}

// TestMembershipResolvesYouTrackIDs checks that a YouTrack group or user id is
// resolved to its Hub id (ringId) before the Hub membership endpoints are called,
// since those answer a YouTrack id with 404/405 (YouTrack 2026.3).
func TestMembershipResolvesYouTrackIDs(t *testing.T) {
	t.Parallel()

	const (
		ytGroupID  = "4-241"
		ytUserID   = "1-7"
		hubGroupID = "22c600f0-a829-45ee-941e-411236e364cf"
		hubUserID  = "3484732c-7dc8-41d3-a386-46b0e2c28729"
	)

	tests := []struct {
		name         string
		groupID      string
		userID       string
		resolveFails int
		remove       bool
		wantErr      bool
		wantPath     string
	}{
		{name: "add resolves youtrack ids", groupID: ytGroupID, userID: ytUserID, wantPath: "/api/usergroups/" + hubGroupID + "/users"},
		{name: "add keeps hub ids", groupID: hubGroupID, userID: hubUserID, wantPath: "/api/usergroups/" + hubGroupID + "/users"},
		{name: "remove resolves youtrack ids", groupID: ytGroupID, userID: ytUserID, remove: true, wantPath: "/api/usergroups/" + hubGroupID + "/users/" + hubUserID},
		{name: "lookup error is returned", groupID: ytGroupID, userID: ytUserID, resolveFails: http.StatusInternalServerError, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var membershipPath string
			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					if tc.resolveFails != 0 {
						w.WriteHeader(tc.resolveFails)
						return
					}
					switch r.URL.Path {
					case "/api/groups/" + ytGroupID:
						// Serves both the ringId lookup and the membership read-back.
						group := map[string]any{"id": ytGroupID, "ringId": hubGroupID, "ownUsers": []map[string]string{}}
						if !tc.remove {
							group["ownUsers"] = []map[string]string{{"id": ytUserID, "ringId": hubUserID}}
						}
						encodeJSON(t, w, group)
					case "/api/users/" + ytUserID:
						encodeJSON(t, w, map[string]string{"id": ytUserID, "ringId": hubUserID})
					default:
						t.Errorf("unexpected lookup: %s", r.URL.Path)
						w.WriteHeader(http.StatusNotFound)
					}
					return
				}

				membershipPath = r.URL.Path
				if r.Method == http.MethodPost {
					body, _ := io.ReadAll(r.Body)
					if !strings.Contains(string(body), hubUserID) {
						t.Errorf("request body does not contain hub user id: %s", body)
					}
				}
				w.WriteHeader(http.StatusNoContent)
			})
			defer server.Close()

			var err error
			if tc.remove {
				err = client.RemoveUserFromGroup(context.Background(), tc.groupID, tc.userID)
			} else {
				err = client.AddUserToGroup(context.Background(), tc.groupID, tc.userID)
			}

			if tc.wantErr {
				if err == nil {
					t.Fatal(errExpectedError)
				}
				return
			}
			if err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
			if membershipPath != tc.wantPath {
				t.Fatalf("membership call went to %q, want %q", membershipPath, tc.wantPath)
			}
		})
	}
}

// --- RemoveUserFromGroup ---

func TestRemoveUserFromGroup(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Fatalf(errUnexpectedMethod, r.Method)
			}
			if !strings.HasSuffix(r.URL.Path, "/api/usergroups/"+testGroupID+"/users/"+testUserID) {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		})
		defer server.Close()

		if err := client.RemoveUserFromGroup(context.Background(), testGroupID, testUserID); err != nil {
			t.Fatalf(fmtUnexpectedError, err)
		}
	})

	t.Run("404 is ignored", func(t *testing.T) {
		t.Parallel()

		client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		defer server.Close()

		if err := client.RemoveUserFromGroup(context.Background(), testGroupID, testUserID); err != nil {
			t.Fatalf(fmtUnexpectedError, err)
		}
	})
}
