package youtrack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

const (
	testAzureModuleID   = "azure-module-1"
	testAzureModuleName = "Test Azure Module"
	testAzureTenant     = "contoso.onmicrosoft.com"
)

func TestCreateAzureAuthModule(t *testing.T) {
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
				encodeJSON(t, w, AzureAuthModule{ID: testAzureModuleID, Name: testAzureModuleName})
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

			got, err := client.CreateAzureAuthModule(context.Background(), AzureAuthModule{Name: testAzureModuleName})
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.ID != testAzureModuleID {
				t.Fatalf(fmtUnexpectedID, got.ID, testAzureModuleID)
			}
		})
	}
}

func TestGetAzureAuthModuleByID(t *testing.T) {
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
				encodeJSON(t, w, AzureAuthModule{ID: testAzureModuleID, Name: testAzureModuleName, Tenant: testAzureTenant})
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

			got, err := client.GetAzureAuthModuleByID(context.Background(), testAzureModuleID)
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.ID != testAzureModuleID {
				t.Fatalf(fmtUnexpectedID, got.ID, testAzureModuleID)
			}
		})
	}
}

// TestUpdateAzureAuthModuleClearsTenant guards clearing tenant back to ""
// (multi-tenant "common" login): the key must be sent explicitly, not
// omitted, or Hub leaves the previously-configured tenant untouched.
func TestUpdateAzureAuthModuleClearsTenant(t *testing.T) {
	t.Parallel()

	var sawPost, sawGet bool

	handler := func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case httpMethodPost:
			sawPost = true
			assertClearedTenantInUpdatePayload(t, r)
			w.WriteHeader(http.StatusOK)
		case httpMethodGet:
			sawGet = true
			encodeJSON(t, w, AzureAuthModule{ID: testAzureModuleID, Name: testAzureModuleName})
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}

	client, server := newTestClient(t, handler)
	defer server.Close()

	got, err := client.UpdateAzureAuthModule(context.Background(), testAzureModuleID, AzureAuthModule{
		Name:   testAzureModuleName,
		Tenant: "",
	})
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if !sawPost || !sawGet {
		t.Fatalf("expected update to POST the change and then GET the refreshed module, sawPost=%v sawGet=%v", sawPost, sawGet)
	}
	if got.Tenant != "" {
		t.Fatalf("expected cleared tenant in refreshed module, got %q", got.Tenant)
	}
}

// assertClearedTenantInUpdatePayload verifies the update request body
// explicitly sends "tenant": "" rather than omitting the key, since Hub
// leaves a previously-configured tenant untouched if the key is absent.
func assertClearedTenantInUpdatePayload(t *testing.T, r *http.Request) {
	t.Helper()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to decode update payload: %v", err)
	}

	v, ok := payload["tenant"]
	if !ok {
		t.Fatal("expected update payload to include cleared field \"tenant\", but the key was omitted")
	}
	if v != "" {
		t.Fatalf("expected update payload field \"tenant\" to be empty, got %q", v)
	}
}

func TestUpdateAzureAuthModule(t *testing.T) {
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
				encodeJSON(t, w, AzureAuthModule{ID: testAzureModuleID, Name: testAzureModuleName, Tenant: testAzureTenant})
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

			got, err := client.UpdateAzureAuthModule(context.Background(), testAzureModuleID, AzureAuthModule{Name: testAzureModuleName})
			if checkErr(t, err, tc.wantErr) {
				return
			}
			if got.Tenant != testAzureTenant {
				t.Fatalf("unexpected tenant: got %q, want %q", got.Tenant, testAzureTenant)
			}
		})
	}
}

func TestDeleteAzureAuthModule(t *testing.T) {
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

			err := client.DeleteAzureAuthModule(context.Background(), testAzureModuleID)
			checkErr(t, err, tc.wantErr)
		})
	}
}
