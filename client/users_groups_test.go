package youtrack

import (
	"context"
	"net/http"
	"testing"
)

const (
	testGroupID         = "group-1"
	testGroupName       = "Dev Team"
	testUserLogin       = "alice"
	testUserID          = "user-1"
	testOtherUserID     = "user-2"
	testOtherUserLogin  = "bob"
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
