package youtrack

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"sync/atomic"
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
		if r.URL.Query().Get("fields") == "" {
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
				assertOwnedBundleNotFound(t, err, tc.notFound)
				return
			}
			if bundle.ID != tc.wantID {
				t.Fatalf(fmtUnexpectedID, bundle.ID, tc.wantID)
			}
		})
	}
}

func assertOwnedBundleNotFound(t *testing.T, err error, notFound bool) {
	t.Helper()

	if notFound && !IsOwnedBundleNotFoundError(err) {
		t.Fatalf("expected owned bundle not-found error, got %v", err)
	}
}

func assertOwnedBundlePayload(t *testing.T, r *http.Request) {
	t.Helper()

	var payload OwnedBundle
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Errorf("failed to decode request body: %v", err)
		return
	}
	if payload.Name != testOwnedBundleName || len(payload.Values) != 1 {
		t.Errorf("unexpected payload: %+v", payload)
		return
	}
	if owner := payload.Values[0].Owner; owner == nil || owner.ID != "1-1" {
		t.Errorf("owner not sent: %+v", owner)
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
				assertOwnedBundlePayload(t, r)
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

func TestOwnedBundleValueRequests(t *testing.T) {
	t.Parallel()

	const valueID = "49-1"
	valuesPath := ownedBundleByIDTestURL + "/values"
	description := "Server side"

	tests := []struct {
		name       string
		wantMethod string
		wantPath   string
		wantBody   map[string]any
		call       func(context.Context, *Client) error
	}{
		{
			name:       "add value sends the element",
			wantMethod: http.MethodPost,
			wantPath:   valuesPath,
			wantBody:   map[string]any{"name": "Backend", "owner": map[string]any{"id": "1-1"}},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.AddOwnedBundleValue(ctx, testOwnedBundleID, OwnedBundleElement{Name: "Backend", Owner: &UserRef{ID: "1-1"}})
				return err
			},
		},
		{
			name:       "update value clears owner and description with null",
			wantMethod: http.MethodPost,
			wantPath:   valuesPath + "/" + valueID,
			wantBody:   map[string]any{"name": "Backend", "description": nil, "archived": false, "owner": nil},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.ReplaceOwnedBundleValue(ctx, testOwnedBundleID, valueID, OwnedBundleValueUpdate{Name: "Backend"})
				return err
			},
		},
		{
			name:       "update value sends every set field",
			wantMethod: http.MethodPost,
			wantPath:   valuesPath + "/" + valueID,
			wantBody: map[string]any{
				"name": "Backend", "description": description, "archived": true, "ordinal": float64(3),
				"owner": map[string]any{"id": "1-1"},
			},
			call: func(ctx context.Context, c *Client) error {
				ordinal := 3
				_, err := c.ReplaceOwnedBundleValue(ctx, testOwnedBundleID, valueID, OwnedBundleValueUpdate{
					Name: "Backend", Description: &description, Archived: true, Ordinal: &ordinal, Owner: &UserRef{ID: "1-1"},
				})
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.wantMethod {
					t.Errorf(errUnexpectedMethod, r.Method)
				}
				if r.URL.Path != tc.wantPath {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}
				assertJSONBody(t, r, tc.wantBody)
				encodeJSON(t, w, OwnedBundleElement{ID: valueID, Name: "Backend"})
			})
			defer server.Close()

			if err := tc.call(context.Background(), client); err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
		})
	}
}

func assertJSONBody(t *testing.T, r *http.Request, want map[string]any) {
	t.Helper()

	if want == nil {
		return
	}

	var got map[string]any
	if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
		t.Errorf("failed to decode request body: %v", err)
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected body:\n got: %v\nwant: %v", got, want)
	}
}

func TestDeleteOwnedBundleValueTreatsNotFoundAsDeleted(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	if err := client.DeleteOwnedBundleValue(context.Background(), testOwnedBundleID, "49-9"); err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
}

// TestDeleteOwnedBundleValueWaitsUntilGone covers YouTrack acknowledging a value
// delete before applying it: the delete must not return while a read still
// lists the value.
func TestDeleteOwnedBundleValueWaitsUntilGone(t *testing.T) {
	t.Parallel()

	const valueID = "49-1"
	var reads atomic.Int32

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			if r.URL.Path != ownedBundleByIDTestURL+"/values/"+valueID {
				t.Errorf(fmtUnexpectedPath, r.URL.Path)
			}
		case http.MethodGet:
			bundle := testOwnedBundle()
			if reads.Add(1) > 1 {
				bundle.Values = bundle.Values[1:]
			}
			encodeJSON(t, w, bundle)
		default:
			t.Errorf(errUnexpectedMethod, r.Method)
		}
	})
	defer server.Close()

	if err := client.DeleteOwnedBundleValue(context.Background(), testOwnedBundleID, valueID); err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if got := reads.Load(); got != 2 {
		t.Fatalf("expected 2 reads before the value was gone, got %d", got)
	}
}
