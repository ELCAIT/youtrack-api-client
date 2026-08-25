package youtrack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const (
	testAppEndpointApp     = "release-manager"
	testAppEndpointHandler = "backend"
	testAppEndpointPath    = "app-settings"
	testAppEndpointProject = "0-0"

	// testAppEndpointURL is the path the project-scoped endpoint must resolve to.
	testAppEndpointURL = testProjectsAPI + "/" + testAppEndpointProject +
		"/extensionEndpoints/" + testAppEndpointApp + "/" + testAppEndpointHandler + "/" + testAppEndpointPath

	fmtUnexpectedEndpointPath = "unexpected path: got %s, want %s"
	fmtUnexpectedBody         = "unexpected body: got %s, want %s"
)

// testAppEndpointRef is the reference used across these tests.
var testAppEndpointRef = AppEndpointRef{
	AppName: testAppEndpointApp,
	Handler: testAppEndpointHandler,
	Path:    testAppEndpointPath,
}

func TestGetProjectAppEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		response   string
		wantErr    bool
		wantResult string
	}{
		{
			name:       "returns raw settings payload",
			status:     http.StatusOK,
			response:   `{"customFieldNames":["State"],"products":[]}`,
			wantResult: `{"customFieldNames":["State"],"products":[]}`,
		},
		{
			name:     "propagates handler error",
			status:   http.StatusBadRequest,
			response: `{"error":"At least one custom field name is required"}`,
			wantErr:  true,
		},
		{
			name:     "propagates not found",
			status:   http.StatusNotFound,
			response: `{"error":"Not Found"}`,
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf(errUnexpectedMethod, r.Method)
				}
				if r.URL.Path != testAppEndpointURL {
					t.Errorf(fmtUnexpectedEndpointPath, r.URL.Path, testAppEndpointURL)
				}

				w.WriteHeader(tc.status)
				if _, err := w.Write([]byte(tc.response)); err != nil {
					t.Fatalf("failed to write response: %v", err)
				}
			})
			defer server.Close()

			got, err := client.GetProjectAppEndpoint(context.Background(), testAppEndpointProject, testAppEndpointRef)
			if tc.wantErr {
				if err == nil {
					t.Fatal(errExpectedError)
				}

				return
			}

			if err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
			if string(got) != tc.wantResult {
				t.Errorf(fmtUnexpectedBody, got, tc.wantResult)
			}
		})
	}
}

func TestPutProjectAppEndpoint(t *testing.T) {
	t.Parallel()

	payload := map[string]any{"customFieldNames": []string{"State"}}
	wantBody := `{"customFieldNames":["State"]}`

	tests := []struct {
		name     string
		status   int
		response string
		wantErr  bool
	}{
		{
			name:     "sends payload and returns response",
			status:   http.StatusOK,
			response: wantBody,
		},
		{
			name:     "propagates validation error",
			status:   http.StatusBadRequest,
			response: `{"error":"At least one custom field name is required"}`,
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf(errUnexpectedMethod, r.Method)
				}
				if r.URL.Path != testAppEndpointURL {
					t.Errorf(fmtUnexpectedEndpointPath, r.URL.Path, testAppEndpointURL)
				}

				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}
				if got := strings.TrimSpace(string(body)); got != wantBody {
					t.Errorf(fmtUnexpectedBody, got, wantBody)
				}

				w.WriteHeader(tc.status)
				if _, err := w.Write([]byte(tc.response)); err != nil {
					t.Fatalf("failed to write response: %v", err)
				}
			})
			defer server.Close()

			got, err := client.PutProjectAppEndpoint(context.Background(), testAppEndpointProject, testAppEndpointRef, payload)
			if tc.wantErr {
				if err == nil {
					t.Fatal(errExpectedError)
				}

				return
			}

			if err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
			if string(got) != tc.response {
				t.Errorf(fmtUnexpectedBody, got, tc.response)
			}
		})
	}
}

// TestPostProjectAppEndpoint checks that POST reaches the endpoint with a body.
func TestPostProjectAppEndpoint(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf(errUnexpectedMethod, r.Method)
		}
		if r.URL.Path != testAppEndpointURL {
			t.Errorf(fmtUnexpectedEndpointPath, r.URL.Path, testAppEndpointURL)
		}

		encodeJSON(t, w, map[string]any{"ok": true})
	})
	defer server.Close()

	got, err := client.PostProjectAppEndpoint(context.Background(), testAppEndpointProject,
		testAppEndpointRef, map[string]any{"refresh": true})
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if decoded["ok"] != true {
		t.Errorf("unexpected response: %v", decoded)
	}
}

// TestGetProjectAppEndpointSendsNoBody guards against sending a JSON body on a
// GET, which some app handlers reject.
func TestGetProjectAppEndpointSendsNoBody(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if len(body) != 0 {
			t.Errorf("expected empty body, got %s", body)
		}

		encodeJSON(t, w, map[string]any{})
	})
	defer server.Close()

	if _, err := client.GetProjectAppEndpoint(context.Background(), testAppEndpointProject, testAppEndpointRef); err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
}

func TestDeleteProjectAppEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "deletes successfully", status: http.StatusOK},
		{name: "treats not found as success", status: http.StatusNotFound},
		{name: "propagates other errors", status: http.StatusBadRequest, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf(errUnexpectedMethod, r.Method)
				}
				if r.URL.Path != testAppEndpointURL {
					t.Errorf(fmtUnexpectedEndpointPath, r.URL.Path, testAppEndpointURL)
				}

				w.WriteHeader(tc.status)
			})
			defer server.Close()

			err := client.DeleteProjectAppEndpoint(context.Background(), testAppEndpointProject, testAppEndpointRef)
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

// TestAppEndpointRefEscapesPathSegments checks that identifiers containing
// characters that are significant in a URL are escaped rather than changing the
// shape of the path.
func TestAppEndpointRefEscapesPathSegments(t *testing.T) {
	t.Parallel()

	ref := AppEndpointRef{AppName: "@vendor/app", Handler: "backend", Path: "app-settings"}
	want := testProjectsAPI + "/0-0/extensionEndpoints/@vendor%2Fapp/backend/app-settings"

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != want {
			t.Errorf(fmtUnexpectedEndpointPath, r.URL.EscapedPath(), want)
		}

		encodeJSON(t, w, map[string]any{})
	})
	defer server.Close()

	if _, err := client.GetProjectAppEndpoint(context.Background(), testAppEndpointProject, ref); err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
}
