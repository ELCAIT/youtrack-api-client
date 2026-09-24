package youtrack

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
)

const (
	testBundleID      = "66-1"
	testBundleValueID = "67-1"
)

// TestEnumAndStateBundleValueRequests checks the per-value calls address the
// right endpoint and that a replace sends null for what it clears.
func TestEnumAndStateBundleValueRequests(t *testing.T) {
	t.Parallel()

	enumValues := "/" + enumBundlesAPIPath + "/" + testBundleID + "/values"
	stateValues := "/" + stateBundlesAPIPath + "/" + testBundleID + "/values"
	description := "Blocks a release"

	tests := []struct {
		name     string
		wantPath string
		wantBody map[string]any
		call     func(context.Context, *Client) error
	}{
		{
			name:     "add enum value",
			wantPath: enumValues,
			wantBody: map[string]any{"name": "Critical"},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.AddEnumBundleValue(ctx, testBundleID, EnumBundleElement{Name: "Critical"})
				return err
			},
		},
		{
			name:     "replace enum value clears localized name and description",
			wantPath: enumValues + "/" + testBundleValueID,
			wantBody: map[string]any{"name": "Critical", "localizedName": nil, "description": nil, "archived": false},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.ReplaceEnumBundleValue(ctx, testBundleID, testBundleValueID, EnumBundleValueUpdate{Name: "Critical"})
				return err
			},
		},
		{
			name:     "add state value",
			wantPath: stateValues,
			wantBody: map[string]any{"name": "Done", "isResolved": true},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.AddStateBundleValue(ctx, testBundleID, StateBundleElement{Name: "Done", IsResolved: true})
				return err
			},
		},
		{
			name:     "replace state value sends every field",
			wantPath: stateValues + "/" + testBundleValueID,
			wantBody: map[string]any{
				"name": "Done", "localizedName": nil, "description": description,
				"isResolved": false, "archived": true, "ordinal": float64(2),
			},
			call: func(ctx context.Context, c *Client) error {
				ordinal := 2
				_, err := c.ReplaceStateBundleValue(ctx, testBundleID, testBundleValueID, StateBundleValueUpdate{
					Name: "Done", Description: &description, Archived: true, Ordinal: &ordinal,
				})
				return err
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
				assertJSONBody(t, r, tc.wantBody)
				encodeJSON(t, w, map[string]string{"id": testBundleValueID})
			})
			defer server.Close()

			if err := tc.call(context.Background(), client); err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
		})
	}
}

// TestEnumAndStateBundleValueDeleteWaitsUntilGone covers YouTrack
// acknowledging a value delete before applying it.
func TestEnumAndStateBundleValueDeleteWaitsUntilGone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		apiPath string
		bundle  func(listed bool) any
		remove  func(context.Context, *Client) error
	}{
		{
			name:    "enum",
			apiPath: enumBundlesAPIPath,
			bundle: func(listed bool) any {
				if listed {
					return EnumBundle{ID: testBundleID, Values: []EnumBundleElement{{ID: testBundleValueID}}}
				}
				return EnumBundle{ID: testBundleID}
			},
			remove: func(ctx context.Context, c *Client) error {
				return c.DeleteEnumBundleValue(ctx, testBundleID, testBundleValueID)
			},
		},
		{
			name:    "state",
			apiPath: stateBundlesAPIPath,
			bundle: func(listed bool) any {
				if listed {
					return StateBundle{ID: testBundleID, Values: []StateBundleElement{{ID: testBundleValueID}}}
				}
				return StateBundle{ID: testBundleID}
			},
			remove: func(ctx context.Context, c *Client) error {
				return c.DeleteStateBundleValue(ctx, testBundleID, testBundleValueID)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reads atomic.Int32
			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodDelete:
					if want := "/" + tc.apiPath + "/" + testBundleID + "/values/" + testBundleValueID; r.URL.Path != want {
						t.Errorf(fmtUnexpectedPath, r.URL.Path)
					}
				case http.MethodGet:
					encodeJSON(t, w, tc.bundle(reads.Add(1) == 1))
				default:
					t.Errorf(errUnexpectedMethod, r.Method)
				}
			})
			defer server.Close()

			if err := tc.remove(context.Background(), client); err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
			if got := reads.Load(); got != 2 {
				t.Fatalf("expected 2 reads before the value was gone, got %d", got)
			}
		})
	}
}

// TestDeleteBundleRetriesWhileInUse covers YouTrack refusing to delete a bundle
// until it has dropped the reference from a field removed just before.
func TestDeleteBundleRetriesWhileInUse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		responses []int
		body      string
		wantCalls int32
		wantErr   bool
	}{
		{name: "in use then deleted", responses: []int{http.StatusBadRequest, http.StatusOK}, body: `{"error":"x","error_description":"This bundle has usages you are not allowed to delete it"}`, wantCalls: 2},
		{name: "other bad request is not retried", responses: []int{http.StatusBadRequest}, body: `{"error":"x","error_description":"Something else"}`, wantCalls: 1, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32
			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf(errUnexpectedMethod, r.Method)
				}
				status := tc.responses[len(tc.responses)-1]
				if n := int(calls.Add(1)); n <= len(tc.responses) {
					status = tc.responses[n-1]
				}
				w.WriteHeader(status)
				if status != http.StatusOK {
					_, _ = w.Write([]byte(tc.body))
				}
			})
			defer server.Close()

			err := client.DeleteOwnedBundle(context.Background(), testBundleID)
			checkErr(t, err, tc.wantErr)
			if got := calls.Load(); got != tc.wantCalls {
				t.Fatalf("expected %d delete calls, got %d", tc.wantCalls, got)
			}
		})
	}
}
