package youtrack

import (
	"context"
	"net/http"
	"testing"
)

const (
	testIssueLinkTypeIDClient             = "80-1"
	testIssueLinkTypeNameClient           = "Depend"
	testIssueLinkTypeSourceToTargetClient = "is required for"
	testIssueLinkTypeTargetToSourceClient = "depends on"
	testCaseServerFailure                 = "returns error on server failure"
)

func TestGetAllIssueLinkTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantLen int
		wantErr bool
	}{
		{
			name: "returns issue link type list",
			handler: func(w http.ResponseWriter, r *http.Request) {
				encodeJSON(t, w, []IssueLinkType{{ID: testIssueLinkTypeIDClient, Name: testIssueLinkTypeNameClient}})
			},
			wantLen: 1,
		},
		{
			name: "returns error on invalid JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(testInvalidJSON))
			},
			wantErr: true,
		},
		{
			name: "returns error on server error",
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

			got, err := client.GetAllIssueLinkTypes(context.Background())
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if len(got) != tc.wantLen {
				t.Fatalf("got %d issue link types, want %d", len(got), tc.wantLen)
			}
		})
	}
}

func TestGetIssueLinkTypeByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantID  string
		wantErr bool
	}{
		{
			name: "returns issue link type by id",
			handler: func(w http.ResponseWriter, r *http.Request) {
				encodeJSON(t, w, IssueLinkType{ID: testIssueLinkTypeIDClient, Name: testIssueLinkTypeNameClient})
			},
			wantID: testIssueLinkTypeIDClient,
		},
		{
			name: "returns error on not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
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

			got, err := client.GetIssueLinkTypeByID(context.Background(), testIssueLinkTypeIDClient)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.ID != tc.wantID {
				t.Fatalf(fmtUnexpectedID, got.ID, tc.wantID)
			}
		})
	}
}

func TestCreateIssueLinkType(t *testing.T) {
	t.Parallel()

	created := IssueLinkType{ID: testIssueLinkTypeIDClient, Name: testIssueLinkTypeNameClient}

	tests := []struct {
		name    string
		input   IssueLinkType
		handler http.HandlerFunc
		wantID  string
		wantErr bool
	}{
		{
			name:  "creates issue link type",
			input: IssueLinkType{Name: testIssueLinkTypeNameClient, SourceToTarget: testIssueLinkTypeSourceToTargetClient, TargetToSource: testIssueLinkTypeTargetToSourceClient},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf(errUnexpectedMethod, r.Method)
				}
				encodeJSON(t, w, created)
			},
			wantID: testIssueLinkTypeIDClient,
		},
		{
			name:  testCaseServerFailure,
			input: IssueLinkType{Name: testIssueLinkTypeNameClient},
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

			got, err := client.CreateIssueLinkType(context.Background(), tc.input)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.ID != tc.wantID {
				t.Fatalf(fmtUnexpectedID, got.ID, tc.wantID)
			}
		})
	}
}

func TestUpdateIssueLinkType(t *testing.T) {
	t.Parallel()

	updated := IssueLinkType{ID: testIssueLinkTypeIDClient, Name: testUpdatedName}

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantID  string
		wantErr bool
	}{
		{
			name: "updates issue link type",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf(errUnexpectedMethod, r.Method)
				}
				encodeJSON(t, w, updated)
			},
			wantID: testIssueLinkTypeIDClient,
		},
		{
			name: testCaseServerFailure,
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

			got, err := client.UpdateIssueLinkType(context.Background(), testIssueLinkTypeIDClient, IssueLinkType{Name: testUpdatedName})
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.ID != tc.wantID {
				t.Fatalf(fmtUnexpectedID, got.ID, tc.wantID)
			}
		})
	}
}

func TestDeleteIssueLinkType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{name: "deletes issue link type", statusCode: http.StatusOK},
		{name: "ignores not found", statusCode: http.StatusNotFound},
		{name: testCaseServerFailure, statusCode: http.StatusInternalServerError, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf(errUnexpectedMethod, r.Method)
				}
				w.WriteHeader(tc.statusCode)
			})
			defer server.Close()

			err := client.DeleteIssueLinkType(context.Background(), testIssueLinkTypeIDClient)
			if tc.wantErr {
				if err == nil {
					t.Fatal(errExpectedError)
				}
				return
			}
			if err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
		})
	}
}
