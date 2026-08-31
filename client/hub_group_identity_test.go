package youtrack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

const (
	testIDPGroupID   = "550e8400-e29b-41d4-a716-446655440200"
	testHubGroupID   = "29f6f650-d2e3-4d17-a1ed-757c7533a113"
	testHubGroupName = "external-collaborator"
)

// --- FindGroupByIDPGroupID ---

func TestFindGroupByIDPGroupID(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/hub/api/rest/usergroups") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		// idpGroupId has to be requested explicitly: Hub omits it otherwise, and the
		// scan would then match nothing on every group.
		if !strings.Contains(r.URL.Query().Get("fields"), "idpGroupId") {
			t.Fatalf("fields query does not request idpGroupId: %s", r.URL.Query().Get("fields"))
		}

		encodeJSON(t, w, hubGroupsPage{UserGroups: []HubGroup{
			{ID: "other", Name: "unrelated", IDPGroupID: "11111111-1111-1111-1111-111111111111", MappedInAuthModule: &AuthModule{Name: testAuthModuleName}},
			{ID: testHubGroupID, Name: testHubGroupName, IDPGroupID: testIDPGroupID, MappedInAuthModule: &AuthModule{Name: testAuthModuleName}},
		}})
	})
	defer server.Close()

	group, err := client.FindGroupByIDPGroupID(context.Background(), testAuthModuleName, testIDPGroupID)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if group == nil {
		t.Fatal("expected a group, got nil")
	}
	if group.ID != testHubGroupID {
		t.Fatalf(fmtUnexpectedID, group.ID, testHubGroupID)
	}
}

// A role with no group yet is a normal state, not a failure, so it is reported as a nil
// group rather than an error.
func TestFindGroupByIDPGroupIDNoMatch(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		encodeJSON(t, w, hubGroupsPage{UserGroups: []HubGroup{
			{ID: "other", Name: "unrelated", IDPGroupID: "11111111-1111-1111-1111-111111111111", MappedInAuthModule: &AuthModule{Name: testAuthModuleName}},
		}})
	})
	defer server.Close()

	group, err := client.FindGroupByIDPGroupID(context.Background(), testAuthModuleName, testIDPGroupID)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if group != nil {
		t.Fatalf("expected no group, got %+v", group)
	}
}

// An untagged group must not be mistaken for a match: every group without an identity
// reports an empty idpGroupId, and matching those would resolve an arbitrary group.
func TestFindGroupByIDPGroupIDIgnoresUntaggedGroups(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		encodeJSON(t, w, hubGroupsPage{UserGroups: []HubGroup{
			{ID: "all-users", Name: "All Users"},
			{ID: "registered", Name: "Registered Users"},
		}})
	})
	defer server.Close()

	group, err := client.FindGroupByIDPGroupID(context.Background(), testAuthModuleName, testIDPGroupID)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if group != nil {
		t.Fatalf("expected no group, got %+v", group)
	}
}

// An instance holds thousands of groups, so a match beyond the first page has to be
// found or most roles would silently resolve to nothing.
func TestFindGroupByIDPGroupIDPaginates(t *testing.T) {
	t.Parallel()

	var pages int
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		pages++
		if r.URL.Query().Get("$skip") == "" {
			full := make([]HubGroup, hubGroupPageSize)
			for i := range full {
				full[i] = HubGroup{ID: fmt.Sprintf("g-%d", i), IDPGroupID: fmt.Sprintf("role-%d", i), MappedInAuthModule: &AuthModule{Name: testAuthModuleName}}
			}
			encodeJSON(t, w, hubGroupsPage{UserGroups: full})
			return
		}

		if _, err := strconv.Atoi(r.URL.Query().Get("$skip")); err != nil {
			t.Fatalf("unexpected $skip: %v", err)
		}
		encodeJSON(t, w, hubGroupsPage{UserGroups: []HubGroup{
			{ID: testHubGroupID, Name: testHubGroupName, IDPGroupID: testIDPGroupID, MappedInAuthModule: &AuthModule{Name: testAuthModuleName}},
		}})
	})
	defer server.Close()

	group, err := client.FindGroupByIDPGroupID(context.Background(), testAuthModuleName, testIDPGroupID)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if group == nil || group.ID != testHubGroupID {
		t.Fatalf("expected %s on the second page, got %+v", testHubGroupID, group)
	}
	if pages != 2 {
		t.Fatalf("expected 2 pages, got %d", pages)
	}
}

func TestFindGroupByIDPGroupIDEmpty(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("expected no request for an empty idp group id")
	})
	defer server.Close()

	if _, err := client.FindGroupByIDPGroupID(context.Background(), testAuthModuleName, " "); err == nil {
		t.Fatal(errExpectedError)
	}
}

// --- SetGroupIdentity ---

func TestSetGroupIdentity(t *testing.T) {
	t.Parallel()

	var wrote bool
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/hub/api/rest/usergroups/"+testHubGroupID) {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}

			var sent HubGroup
			if err := json.Unmarshal(body, &sent); err != nil {
				t.Fatalf("failed to unmarshal request body: %v", err)
			}
			if sent.IDPGroupID != testIDPGroupID {
				t.Fatalf("unexpected idp group id: %s", sent.IDPGroupID)
			}
			if sent.IDPGroupName != testHubGroupName {
				t.Fatalf("unexpected idp group name: %s", sent.IDPGroupName)
			}
			// Without the module the group is tagged for no provider, and its id
			// cannot be told from another directory's.
			if sent.MappedInAuthModule == nil || sent.MappedInAuthModule.ID != testAuthModuleID {
				t.Fatalf("payload does not carry the auth module: %+v", sent.MappedInAuthModule)
			}
			// The write must not carry a name or description: those belong to the
			// YouTrack-side create, and resending them here would let this call
			// overwrite them with empty values.
			if sent.Name != "" || sent.Description != "" {
				t.Fatalf("payload carries fields it must not send: %+v", sent)
			}

			wrote = true
			// Hub answers this write with 200 and an empty body.
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodGet {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		encodeJSON(t, w, HubGroup{
			ID:           testHubGroupID,
			Name:         testHubGroupName,
			IDPGroupID:   testIDPGroupID,
			IDPGroupName: testHubGroupName,
		})
	})
	defer server.Close()

	group, err := client.SetGroupIdentity(context.Background(), testHubGroupID, testAuthModuleID, testIDPGroupID, testHubGroupName)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if !wrote {
		t.Fatal("expected the identity to be written")
	}
	// The empty response body is why the result is read back rather than parsed.
	if group.IDPGroupID != testIDPGroupID {
		t.Fatalf("unexpected idp group id: got %q, want %q", group.IDPGroupID, testIDPGroupID)
	}
}

func TestSetGroupIdentityRejectsIncompleteArguments(t *testing.T) {
	t.Parallel()

	tests := map[string]struct{ groupID, authModuleID, idpGroupID string }{
		"missing group id":       {"", testAuthModuleID, testIDPGroupID},
		"missing auth module id": {testHubGroupID, " ", testIDPGroupID},
		"missing idp group id":   {testHubGroupID, testAuthModuleID, "  "},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(_ http.ResponseWriter, _ *http.Request) {
				t.Fatal("expected no request for incomplete arguments")
			})
			defer server.Close()

			if _, err := client.SetGroupIdentity(context.Background(), test.groupID, test.authModuleID, test.idpGroupID, testHubGroupName); err == nil {
				t.Fatal(errExpectedError)
			}
		})
	}
}

// --- GetHubGroup ---

func TestGetHubGroup(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		encodeJSON(t, w, HubGroup{ID: testHubGroupID, Name: testHubGroupName, IDPGroupID: testIDPGroupID})
	})
	defer server.Close()

	group, err := client.GetHubGroup(context.Background(), testHubGroupID)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if group.IDPGroupID != testIDPGroupID {
		t.Fatalf("unexpected idp group id: %s", group.IDPGroupID)
	}
}

// Every provider writes into the same idpGroupId field, so a group tagged by another
// provider that happens to carry this id must not be mistaken for ours. Observed live:
// an Entra ID group on the instance carries an idpGroupId of its own.
func TestFindGroupByIDPGroupIDIgnoresOtherAuthModules(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		encodeJSON(t, w, hubGroupsPage{UserGroups: []HubGroup{{
			ID:                     "entra-group",
			Name:                   "cloud_ems-dpaas_ext-guest",
			IDPGroupID:             testIDPGroupID,
			ImportedFromAuthModule: &AuthModule{Name: "Microsoft Entra ID"},
		}}})
	})
	defer server.Close()

	group, err := client.FindGroupByIDPGroupID(context.Background(), testAuthModuleName, testIDPGroupID)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if group != nil {
		t.Fatalf("expected no group for another provider's tag, got %+v", group)
	}
}

// An empty module name matches on the id alone, for an instance where only one provider
// tags groups.
func TestFindGroupByIDPGroupIDWithoutModuleMatchesAnyProvider(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		encodeJSON(t, w, hubGroupsPage{UserGroups: []HubGroup{{
			ID:                     "entra-group",
			IDPGroupID:             testIDPGroupID,
			ImportedFromAuthModule: &AuthModule{Name: "Microsoft Entra ID"},
		}}})
	})
	defer server.Close()

	group, err := client.FindGroupByIDPGroupID(context.Background(), "", testIDPGroupID)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if group == nil {
		t.Fatal("expected a match when no module is given")
	}
}

// Hub answers a request for an id it does not know with 200 and some other group's
// record rather than a 404 -- observed live, where a deleted group's id returned an
// unrelated group. Trusting that would read and then write the wrong group.
func TestGetHubGroupRejectsAMismatchedGroup(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		encodeJSON(t, w, HubGroup{ID: "some-other-group", Name: "helpdesk-agents"})
	})
	defer server.Close()

	if _, err := client.GetHubGroup(context.Background(), testHubGroupID); !IsNotFound(err) {
		t.Fatalf("expected a not-found error for a mismatched group, got %v", err)
	}
}

// SetGroupIdentity reads the group back, so the same substitution must not let a write
// report success against a group that is not the one addressed.
func TestSetGroupIdentityRejectsAMismatchedReadBack(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		encodeJSON(t, w, HubGroup{ID: "some-other-group", Name: "helpdesk-agents"})
	})
	defer server.Close()

	if _, err := client.SetGroupIdentity(context.Background(), testHubGroupID, testAuthModuleID, testIDPGroupID, testHubGroupName); !IsNotFound(err) {
		t.Fatalf("expected a not-found error for a mismatched read-back, got %v", err)
	}
}
