package youtrack

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

const (
	testOwnedBundleID      = "48-1"
	testOwnedBundleName    = "Subsystems"
	ownedBundlesTestPath   = "/" + ownedBundlesAPIPath
	ownedBundleByIDTestURL = ownedBundlesTestPath + "/" + testOwnedBundleID
)

func testOwnedBundle() OwnedBundle {
	return OwnedBundle{
		ID:   testOwnedBundleID,
		Name: testOwnedBundleName,
		Values: []OwnedBundleElement{
			{
				ID:    "49-1",
				Name:  "Backend",
				Owner: &UserRef{ID: "1-1", Login: "jane.doe", Name: "Jane Doe", Type: "User"},
				Type:  "OwnedBundleElement",
			},
			{ID: "49-2", Name: "Frontend", Ordinal: 1, Type: "OwnedBundleElement"},
		},
		Type: "OwnedBundle",
	}
}

func TestGetOwnedBundleByID(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf(errUnexpectedMethod, r.Method)
		}
		if r.URL.Path != ownedBundleByIDTestURL {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		if got := r.URL.Query().Get("fields"); got == "" {
			t.Error("fields query parameter missing")
		}
		encodeJSON(t, w, testOwnedBundle())
	})
	defer server.Close()

	bundle, err := client.GetOwnedBundleByID(context.Background(), testOwnedBundleID)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if bundle.ID != testOwnedBundleID {
		t.Fatalf(fmtUnexpectedID, bundle.ID, testOwnedBundleID)
	}
	if len(bundle.Values) != 2 {
		t.Fatalf("unexpected value count: got %d, want 2", len(bundle.Values))
	}
	if owner := bundle.Values[0].Owner; owner == nil || owner.Login != "jane.doe" {
		t.Fatalf("unexpected owner of first value: %+v", owner)
	}
	if bundle.Values[1].Owner != nil {
		t.Fatalf("expected no owner on second value, got %+v", bundle.Values[1].Owner)
	}
}

func TestGetOwnedBundleByName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		lookup   string
		response any
		wantID   string
		wantErr  bool
		notFound bool
	}{
		{name: "exact match", lookup: testOwnedBundleName, response: []OwnedBundle{testOwnedBundle()}, wantID: testOwnedBundleID},
		{name: "case-insensitive match", lookup: "subsystems", response: []OwnedBundle{testOwnedBundle()}, wantID: testOwnedBundleID},
		{name: "wrapped list response", lookup: testOwnedBundleName, response: map[string]any{"bundles": []OwnedBundle{testOwnedBundle()}}, wantID: testOwnedBundleID},
		{name: "missing", lookup: "Other", response: []OwnedBundle{testOwnedBundle()}, wantErr: true, notFound: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != ownedBundlesTestPath {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}
				encodeJSON(t, w, tc.response)
			})
			defer server.Close()

			bundle, err := client.GetOwnedBundleByName(context.Background(), tc.lookup)
			if checkErr(t, err, tc.wantErr) {
				if tc.notFound && !IsOwnedBundleNotFoundError(err) {
					t.Fatalf("expected owned bundle not-found error, got %v", err)
				}
				return
			}
			if bundle.ID != tc.wantID {
				t.Fatalf(fmtUnexpectedID, bundle.ID, tc.wantID)
			}
		})
	}
}

func TestCreateAndUpdateOwnedBundle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		wantPath string
		call     func(context.Context, *Client, OwnedBundle) (*OwnedBundle, error)
	}{
		{
			name:     "create",
			wantPath: ownedBundlesTestPath,
			call: func(ctx context.Context, c *Client, b OwnedBundle) (*OwnedBundle, error) {
				return c.CreateOwnedBundle(ctx, b)
			},
		},
		{
			name:     "update",
			wantPath: ownedBundleByIDTestURL,
			call: func(ctx context.Context, c *Client, b OwnedBundle) (*OwnedBundle, error) {
				return c.UpdateOwnedBundle(ctx, testOwnedBundleID, b)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf(errUnexpectedMethod, r.Method)
				}
				if r.URL.Path != tc.wantPath {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}

				var payload OwnedBundle
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Errorf("failed to decode request body: %v", err)
				}
				if payload.Name != testOwnedBundleName || len(payload.Values) != 1 {
					t.Errorf("unexpected payload: %+v", payload)
				} else if owner := payload.Values[0].Owner; owner == nil || owner.ID != "1-1" {
					t.Errorf("owner not sent: %+v", owner)
				}

				encodeJSON(t, w, testOwnedBundle())
			})
			defer server.Close()

			input := OwnedBundle{
				Name:   testOwnedBundleName,
				Values: []OwnedBundleElement{{Name: "Backend", Owner: &UserRef{ID: "1-1"}}},
			}
			bundle, err := tc.call(context.Background(), client, input)
			if err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
			if bundle.ID != testOwnedBundleID {
				t.Fatalf(fmtUnexpectedID, bundle.ID, testOwnedBundleID)
			}
		})
	}
}

func TestDeleteOwnedBundle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "deleted", status: http.StatusOK},
		{name: "already gone is success", status: http.StatusNotFound},
		{name: "server error", status: http.StatusInternalServerError, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf(errUnexpectedMethod, r.Method)
				}
				if r.URL.Path != ownedBundleByIDTestURL {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}
				w.WriteHeader(tc.status)
			})
			defer server.Close()

			err := client.DeleteOwnedBundle(context.Background(), testOwnedBundleID)
			checkErr(t, err, tc.wantErr)
		})
	}
}
