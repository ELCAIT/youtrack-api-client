package youtrack

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
)

// TestIsNotFoundClassifiesErrors covers the contract callers depend on: absence is
// reported as not-found, and every other failure is not, so that "does not exist" is
// never confused with "could not tell".
func TestIsNotFoundClassifiesErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "bare sentinel", err: ErrNotFound, want: true},
		{name: "wrapped sentinel", err: notFoundf("app with name %q", "Diagram Editor"), want: true},
		{name: "doubly wrapped sentinel", err: fmt.Errorf("enable app: %w", notFoundf("app")), want: true},
		{name: "http 404", err: &HTTPError{StatusCode: http.StatusNotFound, Message: "404"}, want: true},
		{name: "wrapped http 404", err: fmt.Errorf("get project: %w", &HTTPError{StatusCode: http.StatusNotFound, Message: "404"}), want: true},
		{name: "http 500 is not absence", err: &HTTPError{StatusCode: http.StatusInternalServerError, Message: "500"}, want: false},
		{name: "http 403 is not absence", err: &HTTPError{StatusCode: http.StatusForbidden, Message: "403"}, want: false},
		{name: "unrelated error", err: errors.New("connection refused"), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := IsNotFound(tc.err); got != tc.want {
				t.Fatalf("IsNotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestNotFoundfMessage checks the rendered message reads naturally and still carries
// the identifier that was looked up.
func TestNotFoundfMessage(t *testing.T) {
	t.Parallel()

	err := notFoundf("user group with name %q", "onprem_elca-platform_admin")

	const want = `user group with name "onprem_elca-platform_admin": not found`
	if err.Error() != want {
		t.Fatalf("unexpected message:\n got: %s\nwant: %s", err.Error(), want)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("notFoundf result does not wrap ErrNotFound")
	}
}

// TestLookupsReportNotFound is the regression test for the inconsistency this file
// addresses: every by-name lookup that comes up empty must be detectable with the
// same predicate, regardless of which endpoint it went through.
func TestLookupsReportNotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response any
		lookup   func(context.Context, *Client) error
	}{
		{
			name:     "GetUserByLogin",
			response: []Holder{{Id: "user-2", Login: "someone.else"}},
			lookup: func(ctx context.Context, c *Client) error {
				_, err := c.GetUserByLogin(ctx, "missing.user")
				return err
			},
		},
		{
			name:     "GetUserGroupByName",
			response: []Holder{{Id: "group-2", Name: "Other Team"}},
			lookup: func(ctx context.Context, c *Client) error {
				_, err := c.GetUserGroupByName(ctx, "onprem_elca-platform_admin")
				return err
			},
		},
		{
			name:     "GetAppByName",
			response: []App{{ID: "app-2", Name: "Some Other App"}},
			lookup: func(ctx context.Context, c *Client) error {
				_, err := c.GetAppByName(ctx, "Diagram Editor")
				return err
			},
		},
		{
			name:     "GetAllUsersGroup",
			response: []Holder{{Id: "group-2", Name: "Not All Users"}},
			lookup: func(ctx context.Context, c *Client) error {
				_, err := c.GetAllUsersGroup(ctx)
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				encodeJSON(t, w, tc.response)
			})
			defer server.Close()

			err := tc.lookup(context.Background(), client)
			if err == nil {
				t.Fatal(errExpectedError)
			}
			if !IsNotFound(err) {
				t.Fatalf("IsNotFound = false for absent entity; got error: %v", err)
			}
		})
	}
}

// TestLookupsDoNotReportNotFoundOnFailure guards the other direction: a server-side
// failure must never be mistaken for absence, which would let a reconciling caller
// create duplicates or delete live data.
func TestLookupsDoNotReportNotFoundOnFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		lookup func(context.Context, *Client) error
	}{
		{
			name: "GetUserByLogin",
			lookup: func(ctx context.Context, c *Client) error {
				_, err := c.GetUserByLogin(ctx, "someone")
				return err
			},
		},
		{
			name: "GetUserGroupByName",
			lookup: func(ctx context.Context, c *Client) error {
				_, err := c.GetUserGroupByName(ctx, "some_group")
				return err
			},
		},
		{
			name: "GetAppByName",
			lookup: func(ctx context.Context, c *Client) error {
				_, err := c.GetAppByName(ctx, "Diagram Editor")
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			})
			defer server.Close()

			err := tc.lookup(context.Background(), client)
			if err == nil {
				t.Fatal(errExpectedError)
			}
			if IsNotFound(err) {
				t.Fatalf("IsNotFound = true for a 500 response; absence and failure must stay distinct: %v", err)
			}
		})
	}
}

// TestTypedPredicatesDoNotCrossTalk guards that chaining the entity sentinels onto the
// shared ErrNotFound did not make the per-entity predicates match each other's errors.
func TestTypedPredicatesDoNotCrossTalk(t *testing.T) {
	t.Parallel()

	appErr := entityNotFoundf(errAppNotFound, "app with name %q", "x")
	genericErr := notFoundf("something")

	if IsCustomFieldNotFoundError(appErr) {
		t.Error("IsCustomFieldNotFoundError matched an app not-found error")
	}
	if IsEnumBundleNotFoundError(appErr) {
		t.Error("IsEnumBundleNotFoundError matched an app not-found error")
	}
	if IsAppNotFoundError(errCustomFieldNotFound) {
		t.Error("IsAppNotFoundError matched a custom field not-found error")
	}
	if IsAppNotFoundError(genericErr) {
		t.Error("IsAppNotFoundError matched a generic not-found error")
	}
	// The shared predicate must still match all of them.
	for _, err := range []error{appErr, genericErr, errCustomFieldNotFound, errEnumBundleNotFound, errStateBundleNotFound} {
		if !IsNotFound(err) {
			t.Errorf("IsNotFound returned false for %v", err)
		}
	}
}

// TestTypedNotFoundPredicates keeps the per-entity helpers agreeing with IsNotFound,
// so existing callers using them keep working.
func TestTypedNotFoundPredicates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		predicate func(error) bool
		err       error
	}{
		{name: "IsAppNotFoundError", predicate: IsAppNotFoundError, err: entityNotFoundf(errAppNotFound, "app with name %q", "x")},
		{name: "IsCustomFieldNotFoundError", predicate: IsCustomFieldNotFoundError, err: fmt.Errorf("%w: %s", errCustomFieldNotFound, "x")},
		{name: "IsEnumBundleNotFoundError", predicate: IsEnumBundleNotFoundError, err: fmt.Errorf("%w: name '%s'", errEnumBundleNotFound, "x")},
		{name: "IsStateBundleNotFoundError", predicate: IsStateBundleNotFoundError, err: fmt.Errorf("%w: name '%s'", errStateBundleNotFound, "x")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if !tc.predicate(tc.err) {
				t.Fatalf("%s returned false for its own not-found error: %v", tc.name, tc.err)
			}
			if !IsNotFound(tc.err) {
				t.Fatalf("IsNotFound returned false for %s error: %v", tc.name, tc.err)
			}
			if tc.predicate(errors.New("boom")) {
				t.Fatalf("%s returned true for an unrelated error", tc.name)
			}
		})
	}
}
