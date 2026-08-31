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
	testAuthModuleName = "TrustID Int"
	testAuthModuleID   = "module-1"
	testIdentifier     = "fc04d7f2-86a9-4d2a-bc1c-e72b38d76382"
)

// --- ListAuthModules / GetAuthModuleByName ---

func TestGetAuthModuleByName(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/hub/api/rest/authmodules") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		encodeJSON(t, w, hubAuthModulesPage{AuthModules: []AuthModule{
			{ID: "core", Name: "Hub", Type: "CoreauthmoduleJSON"},
			{ID: testAuthModuleID, Name: testAuthModuleName, Type: Oauth2DetailsType},
		}})
	})
	defer server.Close()

	module, err := client.GetAuthModuleByName(context.Background(), testAuthModuleName)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if module.ID != testAuthModuleID {
		t.Fatalf(fmtUnexpectedID, module.ID, testAuthModuleID)
	}
}

// A configured module name that no longer exists must be reported as not found, so a
// caller resolving it at startup fails loudly instead of provisioning nothing forever.
func TestGetAuthModuleByNameNotFound(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		encodeJSON(t, w, hubAuthModulesPage{AuthModules: []AuthModule{{ID: "core", Name: "Hub"}}})
	})
	defer server.Close()

	if _, err := client.GetAuthModuleByName(context.Background(), testAuthModuleName); !IsNotFound(err) {
		t.Fatalf("expected a not-found error, got %v", err)
	}
}

func TestGetAuthModuleByNameEmpty(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("expected no request for an empty module name")
	})
	defer server.Close()

	if _, err := client.GetAuthModuleByName(context.Background(), "  "); err == nil {
		t.Fatal(errExpectedError)
	}
}

// --- FindUserByAuthIdentifier ---

func TestFindUserByAuthIdentifier(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/hub/api/rest/users") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		// The listing must be narrowed to the module, otherwise the scan walks
		// every user in the instance.
		if got, want := r.URL.Query().Get("query"), fmt.Sprintf(hubAuthModuleQueryFormat, testAuthModuleName); got != want {
			t.Fatalf("unexpected query: got %q, want %q", got, want)
		}

		encodeJSON(t, w, hubUsersPage{Users: []HubUser{
			{ID: "user-other", Login: "other", Details: []UserDetail{
				{Type: Oauth2DetailsType, AuthModuleName: testAuthModuleName, Identifier: "11111111-1111-1111-1111-111111111111"},
			}},
			{ID: testUserID, Login: testUserLogin, Details: []UserDetail{
				{Type: Oauth2DetailsType, AuthModuleName: testAuthModuleName, Identifier: testIdentifier},
			}},
		}})
	})
	defer server.Close()

	user, err := client.FindUserByAuthIdentifier(context.Background(), testAuthModuleName, testIdentifier)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if user == nil {
		t.Fatal("expected a user, got nil")
	}
	if user.ID != testUserID {
		t.Fatalf(fmtUnexpectedID, user.ID, testUserID)
	}
}

// A missing identifier is not an error: an event naming an account that YouTrack does
// not have is a normal outcome, and the caller distinguishes it by the nil user.
func TestFindUserByAuthIdentifierNoMatch(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		encodeJSON(t, w, hubUsersPage{Users: []HubUser{
			{ID: "user-other", Login: "other", Details: []UserDetail{
				{Type: Oauth2DetailsType, AuthModuleName: testAuthModuleName, Identifier: "11111111-1111-1111-1111-111111111111"},
			}},
		}})
	})
	defer server.Close()

	user, err := client.FindUserByAuthIdentifier(context.Background(), testAuthModuleName, testIdentifier)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if user != nil {
		t.Fatalf("expected no user, got %+v", user)
	}
}

// Identifiers of different providers are unrelated namespaces, so a value that matches
// another module's detail must not be mistaken for this module's user.
func TestFindUserByAuthIdentifierIgnoresOtherModules(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		encodeJSON(t, w, hubUsersPage{Users: []HubUser{
			{ID: "user-entra", Login: "entra", Details: []UserDetail{
				{Type: "AzuredetailsJSON", AuthModuleName: "Microsoft Entra ID", Identifier: testIdentifier},
			}},
		}})
	})
	defer server.Close()

	user, err := client.FindUserByAuthIdentifier(context.Background(), testAuthModuleName, testIdentifier)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if user != nil {
		t.Fatalf("expected no user, got %+v", user)
	}
}

// The match must survive past the first page, otherwise a module with more users than
// one page silently stops resolving anyone beyond it.
func TestFindUserByAuthIdentifierPaginates(t *testing.T) {
	t.Parallel()

	var pages int
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		skip, err := strconv.Atoi(r.URL.Query().Get("$skip"))
		if err != nil && r.URL.Query().Get("$skip") != "" {
			t.Fatalf("unexpected $skip: %v", err)
		}
		pages++

		if skip == 0 {
			full := make([]HubUser, hubUserDetailPageSize)
			for i := range full {
				full[i] = HubUser{ID: fmt.Sprintf("user-%d", i), Details: []UserDetail{
					{Type: Oauth2DetailsType, AuthModuleName: testAuthModuleName, Identifier: fmt.Sprintf("id-%d", i)},
				}}
			}
			encodeJSON(t, w, hubUsersPage{Users: full})
			return
		}

		encodeJSON(t, w, hubUsersPage{Users: []HubUser{
			{ID: testUserID, Login: testUserLogin, Details: []UserDetail{
				{Type: Oauth2DetailsType, AuthModuleName: testAuthModuleName, Identifier: testIdentifier},
			}},
		}})
	})
	defer server.Close()

	user, err := client.FindUserByAuthIdentifier(context.Background(), testAuthModuleName, testIdentifier)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if user == nil || user.ID != testUserID {
		t.Fatalf("expected %s on the second page, got %+v", testUserID, user)
	}
	if pages != 2 {
		t.Fatalf("expected 2 pages, got %d", pages)
	}
}

func TestFindUserByAuthIdentifierEmpty(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("expected no request for an empty identifier")
	})
	defer server.Close()

	if _, err := client.FindUserByAuthIdentifier(context.Background(), testAuthModuleName, " "); err == nil {
		t.Fatal(errExpectedError)
	}
}

// --- AddUserDetail ---

func TestAddUserDetail(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/hub/api/rest/users/"+testUserID+"/userdetails") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var sent UserDetail
		if err := json.Unmarshal(body, &sent); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}
		// Hub selects the subtype from the discriminator and the module from the
		// id, so both must survive marshalling.
		if sent.Type != Oauth2DetailsType {
			t.Fatalf("unexpected type: %s", sent.Type)
		}
		if sent.AuthModule == nil || sent.AuthModule.ID != testAuthModuleID {
			t.Fatalf("unexpected auth module: %+v", sent.AuthModule)
		}
		if sent.Identifier != testIdentifier {
			t.Fatalf("unexpected identifier: %s", sent.Identifier)
		}

		encodeJSON(t, w, UserDetail{
			ID:             "detail-1",
			Type:           Oauth2DetailsType,
			Identifier:     testIdentifier,
			AuthModuleName: testAuthModuleName,
		})
	})
	defer server.Close()

	created, err := client.AddUserDetail(context.Background(), testUserID, UserDetail{
		Type:       Oauth2DetailsType,
		Identifier: testIdentifier,
		AuthModule: &AuthModule{ID: testAuthModuleID},
	})
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if created.ID != "detail-1" {
		t.Fatalf(fmtUnexpectedID, created.ID, "detail-1")
	}
}

// The subtype and the module are what make a detail addressable; without either, Hub
// would reject the write, so the client refuses before spending a round trip.
func TestAddUserDetailRejectsIncompletePayload(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		userID string
		detail UserDetail
	}{
		"missing user id": {"", UserDetail{Type: Oauth2DetailsType, AuthModule: &AuthModule{ID: testAuthModuleID}}},
		"missing type":    {testUserID, UserDetail{AuthModule: &AuthModule{ID: testAuthModuleID}}},
		"missing module":  {testUserID, UserDetail{Type: Oauth2DetailsType}},
		"empty module id": {testUserID, UserDetail{Type: Oauth2DetailsType, AuthModule: &AuthModule{}}},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(_ http.ResponseWriter, _ *http.Request) {
				t.Fatal("expected no request for an incomplete payload")
			})
			defer server.Close()

			if _, err := client.AddUserDetail(context.Background(), test.userID, test.detail); err == nil {
				t.Fatal(errExpectedError)
			}
		})
	}
}

// --- ListUserDetails / RemoveUserDetail ---

func TestListUserDetails(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}

		encodeJSON(t, w, struct {
			UserDetails []UserDetail `json:"userdetails"`
		}{UserDetails: []UserDetail{
			{ID: "detail-1", Type: Oauth2DetailsType, Identifier: testIdentifier, AuthModuleName: testAuthModuleName},
		}})
	})
	defer server.Close()

	details, err := client.ListUserDetails(context.Background(), testUserID)
	if err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
	if len(details) != 1 || details[0].Identifier != testIdentifier {
		t.Fatalf("unexpected details: %+v", details)
	}
}

func TestRemoveUserDetail(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf(errUnexpectedMethod, r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/hub/api/rest/users/"+testUserID+"/userdetails/detail-1") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	if err := client.RemoveUserDetail(context.Background(), testUserID, "detail-1"); err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
}

// A detail that is already gone leaves nothing to undo, so a repeated cleanup settles
// rather than failing.
func TestRemoveUserDetailAlreadyGone(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	if err := client.RemoveUserDetail(context.Background(), testUserID, "detail-1"); err != nil {
		t.Fatalf(fmtUnexpectedError, err)
	}
}
