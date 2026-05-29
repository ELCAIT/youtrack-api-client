package youtrack

import (
	"context"
	"io"
	"net/http"
	"strings"
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
	}{
		{
			name:           "success sends DELETE with successor param",
			statusCode:     http.StatusOK,
			wantDeleteCall: true,
		},
		{
			name:       "404 is silently ignored",
			statusCode: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deleteCalled := false

			client, server := newTestClient(t, newDeleteGroupHandler(t, tc.statusCode, &deleteCalled))
			defer server.Close()

			err := client.DeleteGroup(context.Background(), testGroupID, testAllUsersGroupID)
			if err != nil {
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

// --- DeleteUser ---

func TestDeleteUser(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				encodeJSON(t, w, []Holder{{Id: "guest-id", Login: "guest"}})
				return
			}
			if r.Method != http.MethodDelete {
				t.Fatalf(errUnexpectedMethod, r.Method)
			}
			if !strings.HasSuffix(r.URL.Path, "/api/users/"+testUserID) {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			if r.URL.Query().Get("successor") == "" {
				t.Fatal("expected successor query parameter")
			}
			w.WriteHeader(http.StatusNoContent)
		})
		defer server.Close()

		if err := client.DeleteUser(context.Background(), testUserID); err != nil {
			t.Fatalf(fmtUnexpectedError, err)
		}
	})

	t.Run("404 is ignored", func(t *testing.T) {
		t.Parallel()

		client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				encodeJSON(t, w, []Holder{{Id: "guest-id", Login: "guest"}})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})
		defer server.Close()

		if err := client.DeleteUser(context.Background(), testUserID); err != nil {
			t.Fatalf(fmtUnexpectedError, err)
		}
	})
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
