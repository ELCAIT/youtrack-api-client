package youtrack

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

const (
	testAssignedRoleID = "31-70"
	testGlobalScopeID  = "544-0"
	testProjectID      = "0-2"
	testHolderID       = "4-3"
)

// --- GetAllAssignedRoles ---

func TestGetAllAssignedRoles(t *testing.T) {
	t.Parallel()

	assignedRole := AssignedRole{
		Id:     testAssignedRoleID,
		Role:   Role{Id: testRoleID, Name: testRoleName},
		Scope:  Scope{Id: testGlobalScopeID, Type: "GlobalScope"},
		Holder: Holder{Id: testHolderID, Type: "User"},
		Type:   assignedRoleType,
	}

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantLen int
		wantErr bool
	}{
		{
			name: "returns a bare array of assigned roles",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				encodeJSON(t, w, []AssignedRole{assignedRole})
			},
			wantLen: 1,
		},
		{
			name: "returns error on server failure",
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

			got, err := client.GetAllAssignedRoles(context.Background(), 0, 0)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if len(got) != tc.wantLen {
				t.Fatalf("got %d assigned roles, want %d", len(got), tc.wantLen)
			}
		})
	}
}

// --- GetAssignedRolesByHolder ---

func TestGetAssignedRolesByHolder(t *testing.T) {
	t.Parallel()

	handler := func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got, want := q.Get("query"), "holder:"+testHolderID; got != want {
			t.Errorf("query param = %q, want %q", got, want)
		}
		if got, want := q.Get("$top"), "5"; got != want {
			t.Errorf("$top param = %q, want %q", got, want)
		}
		if got, want := q.Get("$skip"), "10"; got != want {
			t.Errorf("$skip param = %q, want %q", got, want)
		}
		encodeJSON(t, w, []AssignedRole{{Id: testAssignedRoleID}})
	}

	client, server := newTestClient(t, handler)
	defer server.Close()

	got, err := client.GetAssignedRolesByHolder(context.Background(), testHolderID, 5, 10)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d assigned roles, want 1", len(got))
	}
}

// --- GetAssignedRoleById ---

func TestGetAssignedRoleById(t *testing.T) {
	t.Parallel()

	assignedRole := AssignedRole{
		Id:    testAssignedRoleID,
		Role:  Role{Id: testRoleID, Name: testRoleName},
		Scope: Scope{Id: testGlobalScopeID, Type: "GlobalScope"},
	}

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantID  string
		wantErr bool
	}{
		{
			name: "returns assigned role by id",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				encodeJSON(t, w, assignedRole)
			},
			wantID: testAssignedRoleID,
		},
		{
			name: "returns error on 404",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, tc.handler)
			defer server.Close()

			got, err := client.GetAssignedRoleById(context.Background(), testAssignedRoleID)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.Id != tc.wantID {
				t.Fatalf(fmtUnexpectedID, got.Id, tc.wantID)
			}
		})
	}
}

// --- CreateAssignedRole ---

// checkAssignedRoleType asserts that Create/UpdateAssignedRole forced $type on the payload.
func checkAssignedRoleType(t *testing.T, body AssignedRole) {
	t.Helper()

	if body.Type != assignedRoleType {
		t.Errorf("$type = %q, want %q", body.Type, assignedRoleType)
	}
}

// checkGlobalScope asserts the payload carries a bare GlobalScope with no project.
func checkGlobalScope(t *testing.T, scope Scope) {
	t.Helper()

	if scope.Type != "GlobalScope" {
		t.Errorf("scope.$type = %q, want GlobalScope", scope.Type)
	}
	if scope.Project != nil {
		t.Errorf("scope.project = %+v, want nil", scope.Project)
	}
}

// checkProjectScope asserts the payload carries a ProjectScope for the given project.
func checkProjectScope(t *testing.T, scope Scope, projectID string) {
	t.Helper()

	if scope.Type != "ProjectScope" {
		t.Errorf("scope.$type = %q, want ProjectScope", scope.Type)
	}
	if scope.Project == nil || scope.Project.ID != projectID {
		t.Errorf("scope.project = %+v, want project id %q", scope.Project, projectID)
	}
}

// newCreateAssignedRoleHandler returns a handler that validates the create request
// via checkBody and echoes the created assigned role back.
func newCreateAssignedRoleHandler(t *testing.T, checkBody func(t *testing.T, body AssignedRole)) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf(errUnexpectedMethod, r.Method)
		}

		var body AssignedRole
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		checkBody(t, body)

		body.Id = testAssignedRoleID
		encodeJSON(t, w, body)
	}
}

func TestCreateAssignedRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     AssignedRole
		checkBody func(t *testing.T, body AssignedRole)
	}{
		{
			name: "assigns a role at global scope",
			input: AssignedRole{
				Role:   Role{Id: testRoleID},
				Scope:  Scope{Type: "GlobalScope"},
				Holder: Holder{Id: testHolderID, Type: "User"},
			},
			checkBody: func(t *testing.T, body AssignedRole) {
				checkAssignedRoleType(t, body)
				checkGlobalScope(t, body.Scope)
			},
		},
		{
			name: "assigns a role scoped to a project",
			input: AssignedRole{
				Role:   Role{Id: testRoleID},
				Scope:  Scope{Type: "ProjectScope", Project: &Project{ID: testProjectID}},
				Holder: Holder{Id: testHolderID, Type: "User"},
			},
			checkBody: func(t *testing.T, body AssignedRole) {
				checkAssignedRoleType(t, body)
				checkProjectScope(t, body.Scope, testProjectID)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, newCreateAssignedRoleHandler(t, tc.checkBody))
			defer server.Close()

			got, err := client.CreateAssignedRole(context.Background(), tc.input)
			if err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
			if got.Id != testAssignedRoleID {
				t.Fatalf(fmtUnexpectedID, got.Id, testAssignedRoleID)
			}
		})
	}
}

// --- UpdateAssignedRole ---

func TestUpdateAssignedRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "updates the role assignment",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf(errUnexpectedMethod, r.Method)
				}

				var body AssignedRole
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}
				if body.Type != assignedRoleType {
					t.Errorf("$type = %q, want %q", body.Type, assignedRoleType)
				}

				body.Id = testAssignedRoleID
				encodeJSON(t, w, body)
			},
		},
		{
			name: "returns error on server failure",
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

			input := AssignedRole{
				Role:  Role{Id: testRoleID},
				Scope: Scope{Type: "ProjectScope", Project: &Project{ID: testProjectID}},
			}

			got, err := client.UpdateAssignedRole(context.Background(), testAssignedRoleID, input)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.Id != testAssignedRoleID {
				t.Fatalf(fmtUnexpectedID, got.Id, testAssignedRoleID)
			}
		})
	}
}

// --- DeleteAssignedRole ---

func TestDeleteAssignedRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "deletes the assignment",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf(errUnexpectedMethod, r.Method)
				}
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "idempotent on 404",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
		},
		{
			name: "returns error on server failure",
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

			err := client.DeleteAssignedRole(context.Background(), testAssignedRoleID)
			checkErr(t, err, tc.wantErr)
		})
	}
}
