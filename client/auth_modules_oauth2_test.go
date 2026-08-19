package youtrack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

const (
	testOAuth2ModuleID   = "auth-module-1"
	testOAuth2ModuleName = "Test OAuth2 Module"
	testOAuth2ExtGrant   = "keycloak-extension"
)

func TestCreateOAuth2AuthModule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "creates module and returns response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodPost {
					t.Fatalf("unexpected method: got %s, want %s", r.Method, httpMethodPost)
				}
				encodeJSON(t, w, OAuth2AuthModule{ID: testOAuth2ModuleID, Name: testOAuth2ModuleName})
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

			got, err := client.CreateOAuth2AuthModule(context.Background(), OAuth2AuthModule{Name: testOAuth2ModuleName})
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.ID != testOAuth2ModuleID {
				t.Fatalf(fmtUnexpectedID, got.ID, testOAuth2ModuleID)
			}
		})
	}
}

func TestGetOAuth2AuthModuleByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "returns module by id",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodGet {
					t.Fatalf("unexpected method: got %s, want %s", r.Method, httpMethodGet)
				}
				encodeJSON(t, w, OAuth2AuthModule{ID: testOAuth2ModuleID, Name: testOAuth2ModuleName})
			},
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

			got, err := client.GetOAuth2AuthModuleByID(context.Background(), testOAuth2ModuleID)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.ID != testOAuth2ModuleID {
				t.Fatalf(fmtUnexpectedID, got.ID, testOAuth2ModuleID)
			}
		})
	}
}

// clearedOAuth2Fields are the optional payload keys that must be sent as
// explicit empty values when cleared, rather than omitted.
var clearedOAuth2Fields = []string{"extensionGrantType", "userPictureIdPath"}

// readRequestBody reads a request body in full, failing the test on error.
func readRequestBody(t *testing.T, r *http.Request) []byte {
	t.Helper()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}

	return body
}

// assertFieldsClearedExplicitly asserts that the update payload carries each
// named field as an explicit empty value, rather than dropping the key — which
// is what would silently leave the previous value in place on the Hub side.
func assertFieldsClearedExplicitly(t *testing.T, body []byte, fields []string) {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to decode update payload: %v", err)
	}

	for _, field := range fields {
		value, ok := payload[field]
		if !ok {
			t.Fatalf("expected update payload to include cleared field %q, but the key was omitted", field)
		}
		if value != "" {
			t.Fatalf("expected update payload field %q to be empty, got %q", field, value)
		}
	}
}

// TestUpdateOAuth2AuthModuleClearsOptionalFields guards the fix for
// https://github.com/ELCAIT/terraform-provider-youtrack/issues/39: clearing an
// optional string field (e.g. extension_grant_type, user_picture_id_path) must
// send an explicit empty value on update, not omit the key, otherwise Hub
// leaves the previously-set value untouched.
func TestUpdateOAuth2AuthModuleClearsOptionalFields(t *testing.T) {
	t.Parallel()

	var sawPost, sawGet bool

	handler := func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case httpMethodPost:
			sawPost = true
			assertFieldsClearedExplicitly(t, readRequestBody(t, r), clearedOAuth2Fields)
			w.WriteHeader(http.StatusOK)
		case httpMethodGet:
			sawGet = true
			// Once the update payload confirms the fields were sent as
			// cleared, the refetch can report them as cleared too.
			encodeJSON(t, w, OAuth2AuthModule{ID: testOAuth2ModuleID, Name: testOAuth2ModuleName})
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}

	client, server := newTestClient(t, handler)
	defer server.Close()

	got, err := client.UpdateOAuth2AuthModule(context.Background(), testOAuth2ModuleID, OAuth2AuthModule{
		Name:               testOAuth2ModuleName,
		ExtensionGrantType: "",
		UserPictureIDPath:  "",
	})
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if !sawPost || !sawGet {
		t.Fatalf("expected update to POST the change and then GET the refreshed module, sawPost=%v sawGet=%v", sawPost, sawGet)
	}
	if got.ExtensionGrantType != "" || got.UserPictureIDPath != "" {
		t.Fatalf("expected cleared fields in refreshed module, got extensionGrantType=%q userPictureIdPath=%q",
			got.ExtensionGrantType, got.UserPictureIDPath)
	}
}

func TestUpdateOAuth2AuthModule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "updates module and refetches the result",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == httpMethodPost {
					w.WriteHeader(http.StatusOK)
					return
				}
				encodeJSON(t, w, OAuth2AuthModule{ID: testOAuth2ModuleID, Name: testOAuth2ModuleName, ExtensionGrantType: testOAuth2ExtGrant})
			},
		},
		{
			name: "returns error when the update request fails",
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

			got, err := client.UpdateOAuth2AuthModule(context.Background(), testOAuth2ModuleID, OAuth2AuthModule{Name: testOAuth2ModuleName})
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.ExtensionGrantType != testOAuth2ExtGrant {
				t.Fatalf("unexpected extensionGrantType: got %q, want %q", got.ExtensionGrantType, testOAuth2ExtGrant)
			}
		})
	}
}

func TestDeleteOAuth2AuthModule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "deletes module",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodDelete {
					t.Fatalf("unexpected method: got %s, want %s", r.Method, httpMethodDelete)
				}
				w.WriteHeader(http.StatusOK)
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

			err := client.DeleteOAuth2AuthModule(context.Background(), testOAuth2ModuleID)
			checkErr(t, err, tc.wantErr)
		})
	}
}
