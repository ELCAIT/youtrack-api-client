package youtrack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

const (
	testServiceID   = "service-1"
	testServiceName = "Test Service"
	testServiceKey  = "test-service-key"
)

func TestCreateService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "creates service and returns response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodPost {
					t.Fatalf("unexpected method: got %s, want %s", r.Method, httpMethodPost)
				}
				encodeJSON(t, w, Service{ID: testServiceID, Name: testServiceName, Key: testServiceKey})
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

			got, err := client.CreateService(context.Background(), Service{Name: testServiceName})
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.ID != testServiceID {
				t.Fatalf(fmtUnexpectedID, got.ID, testServiceID)
			}
		})
	}
}

func TestListServices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
		wantLen int
	}{
		{
			name: "returns wrapped services list",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodGet {
					t.Fatalf("unexpected method: got %s, want %s", r.Method, httpMethodGet)
				}
				encodeJSON(t, w, ServicesResponse{Services: []Service{
					{ID: testServiceID, Name: testServiceName},
					{ID: "service-2", Name: "Other Service"},
				}})
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, tc.handler)
			defer server.Close()

			got, err := client.ListServices(context.Background(), 0, 0)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if len(got) != tc.wantLen {
				t.Fatalf("unexpected length: got %d, want %d", len(got), tc.wantLen)
			}
		})
	}
}

func TestGetServiceByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "returns service by id",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodGet {
					t.Fatalf("unexpected method: got %s, want %s", r.Method, httpMethodGet)
				}
				encodeJSON(t, w, Service{ID: testServiceID, Name: testServiceName, Key: testServiceKey})
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

			got, err := client.GetServiceByID(context.Background(), testServiceID)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.ID != testServiceID {
				t.Fatalf(fmtUnexpectedID, got.ID, testServiceID)
			}
		})
	}
}

func TestUpdateService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "updates service and refetches the result",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == httpMethodPost {
					w.WriteHeader(http.StatusOK)
					return
				}
				encodeJSON(t, w, Service{ID: testServiceID, Name: testServiceName, Key: testServiceKey})
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

			got, err := client.UpdateService(context.Background(), testServiceID, Service{Name: testServiceName})
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.Key != testServiceKey {
				t.Fatalf("unexpected key: got %q, want %q", got.Key, testServiceKey)
			}
		})
	}
}

// TestUpdateServiceClearsDescription guards clearing description back to ""
// and redirectUris back to []: both keys must be sent explicitly, not
// omitted, or Hub leaves the previously-configured values untouched.
func TestUpdateServiceClearsDescription(t *testing.T) {
	t.Parallel()

	var sawPost, sawGet bool

	handler := func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case httpMethodPost:
			sawPost = true
			assertClearedFieldsInServiceUpdatePayload(t, r)
			w.WriteHeader(http.StatusOK)
		case httpMethodGet:
			sawGet = true
			encodeJSON(t, w, Service{ID: testServiceID, Name: testServiceName})
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}

	client, server := newTestClient(t, handler)
	defer server.Close()

	got, err := client.UpdateService(context.Background(), testServiceID, Service{
		Name:         testServiceName,
		Description:  "",
		RedirectURIs: []string{},
	})
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if !sawPost || !sawGet {
		t.Fatalf("expected update to POST the change and then GET the refreshed service, sawPost=%v sawGet=%v", sawPost, sawGet)
	}
	if got.Description != "" {
		t.Fatalf("expected cleared description in refreshed service, got %q", got.Description)
	}
}

// assertClearedFieldsInServiceUpdatePayload verifies the update request body
// explicitly sends "description":"" and "redirectUris":[] rather than
// omitting the keys, since Hub leaves previously-configured values untouched
// if the keys are absent.
func assertClearedFieldsInServiceUpdatePayload(t *testing.T, r *http.Request) {
	t.Helper()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to decode update payload: %v", err)
	}

	desc, ok := payload["description"]
	if !ok {
		t.Fatal("expected update payload to include cleared field \"description\", but the key was omitted")
	}
	if desc != "" {
		t.Fatalf("expected update payload field \"description\" to be empty, got %q", desc)
	}

	redirectURIs, ok := payload["redirectUris"]
	if !ok {
		t.Fatal("expected update payload to include cleared field \"redirectUris\", but the key was omitted")
	}
	if list, ok := redirectURIs.([]any); !ok || len(list) != 0 {
		t.Fatalf("expected update payload field \"redirectUris\" to be an empty array, got %v", redirectURIs)
	}
}

// TestCreateServiceSendsOAuthFlowFlags guards that the five OAuth flow bools
// are always sent as explicit true/false in the create payload - they are
// plain (non-pointer) bools, so Go's zero value already marshals as an
// explicit "false" rather than being omitted, unlike the omitempty string
// fields on Service.
func TestCreateServiceSendsOAuthFlowFlags(t *testing.T) {
	t.Parallel()

	var payload map[string]any

	handler := func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf(fmtUnexpectedError, err)
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode create payload: %v", err)
		}
		encodeJSON(t, w, Service{ID: testServiceID, Name: testServiceName})
	}

	client, server := newTestClient(t, handler)
	defer server.Close()

	_, err := client.CreateService(context.Background(), Service{
		Name:                         testServiceName,
		ClientCredentialsFlowEnabled: true,
		AuthCodeFlowEnabled:          true,
		PKCERequired:                 false,
		ImplicitFlowEnabled:          false,
		ResourceOwnerFlowEnabled:     false,
	})
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}

	wantFlags := map[string]bool{
		"clientCredentialsFlowEnabled": true,
		"authCodeFlowEnabled":          true,
		"pkceRequired":                 false,
		"implicitFlowEnabled":          false,
		"resourceOwnerFlowEnabled":     false,
	}
	for field, want := range wantFlags {
		got, ok := payload[field]
		if !ok {
			t.Fatalf("expected create payload to include field %q, but the key was omitted", field)
		}
		if got != want {
			t.Fatalf("expected create payload field %q to be %v, got %v", field, want, got)
		}
	}
}

func TestDeleteService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "deletes service",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodDelete {
					t.Fatalf("unexpected method: got %s, want %s", r.Method, httpMethodDelete)
				}
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "treats not found as success",
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

			err := client.DeleteService(context.Background(), testServiceID)
			checkErr(t, err, tc.wantErr)
		})
	}
}
