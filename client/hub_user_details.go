package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	hubRestAuthModulesAPIPath = "hub/api/rest/authmodules"

	// hubUserDetailFields is the projection needed to reconcile an external identity:
	// the account, and every detail with the module that produced it.
	//
	// userName and fullName are part of it because Hub returns only what is asked for:
	// without them every detail decodes with empty names, and a caller cannot tell a
	// detail that carries names from one that does not -- which is what a backfill has
	// to decide.
	hubUserDetailFields = "id,login,banned,details(" + hubUserDetailOwnFields + ")"

	// hubUserDetailOwnFields is the projection of a user detail on its own, used
	// wherever details are read or written outside the account listing.
	hubUserDetailOwnFields = "id,$type,identifier,authModuleName,userName,fullName"

	// hubAuthModuleFields is the projection needed to resolve a module by name.
	hubAuthModuleFields = "id,name,$type"

	// hubRestUserDetailsAPIPath addresses a user detail on its own. Hub declares only
	// GET and DELETE on the detail under its owning user, so a write goes here.
	hubRestUserDetailsAPIPath = "hub/api/rest/userdetails"

	// hubUserDetailPageSize is the page size used when scanning the users of one
	// authentication module.
	hubUserDetailPageSize = 100

	// hubAuthModuleQueryFormat filters users down to those known to one
	// authentication module. Hub matches the module by name here; braces quote a
	// name containing spaces.
	hubAuthModuleQueryFormat = "authModule: {%s}"
)

// hubUsersPage is one page of the Hub users listing.
type hubUsersPage struct {
	Users []HubUser `json:"users"`
	Total int       `json:"total"`
}

// hubAuthModulesPage is one page of the Hub authentication modules listing.
type hubAuthModulesPage struct {
	AuthModules []AuthModule `json:"authmodules"`
}

// ListAuthModules returns the authentication modules configured in Hub.
func (c *Client) ListAuthModules(ctx context.Context) ([]AuthModule, error) {
	values := url.Values{}
	values.Set("fields", hubAuthModuleFields)

	endpoint := fmt.Sprintf("%s/%s?%s", c.HostURL, hubRestAuthModulesAPIPath, values.Encode())
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list auth modules request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list auth modules: %w", err)
	}

	var page hubAuthModulesPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("failed to unmarshal list auth modules response: %w", err)
	}

	return page.AuthModules, nil
}

// GetAuthModuleByName returns the authentication module with the given name, matched
// case-insensitively. It reports a not-found error when no module matches, so a caller
// resolving a configured module name can fail at startup rather than silently doing
// nothing on every later call.
func (c *Client) GetAuthModuleByName(ctx context.Context, name string) (*AuthModule, error) {
	wanted := strings.TrimSpace(name)
	if wanted == "" {
		return nil, fmt.Errorf("auth module name must not be empty")
	}

	modules, err := c.ListAuthModules(ctx)
	if err != nil {
		return nil, err
	}

	for i := range modules {
		if strings.EqualFold(strings.TrimSpace(modules[i].Name), wanted) {
			return &modules[i], nil
		}
	}

	return nil, notFoundf("auth module with name %q", wanted)
}

// ListUsersByAuthModule returns the Hub users known to one authentication module,
// each with its authentication details.
//
// The module is named rather than addressed by id because Hub's user query supports
// only the name.
func (c *Client) ListUsersByAuthModule(ctx context.Context, authModuleName string, top, skip int) ([]HubUser, error) {
	wanted := strings.TrimSpace(authModuleName)
	if wanted == "" {
		return nil, fmt.Errorf("auth module name must not be empty")
	}

	values := url.Values{}
	values.Set("fields", hubUserDetailFields)
	values.Set("query", fmt.Sprintf(hubAuthModuleQueryFormat, wanted))
	if top > 0 {
		values.Set("$top", strconv.Itoa(top))
	}
	if skip > 0 {
		values.Set("$skip", strconv.Itoa(skip))
	}

	endpoint := fmt.Sprintf("%s/%s?%s", c.HostURL, hubRestUsersAPIPath, values.Encode())
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list users by auth module request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list users by auth module: %w", err)
	}

	var page hubUsersPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("failed to unmarshal list users by auth module response: %w", err)
	}

	return page.Users, nil
}

// FindUserByAuthIdentifier returns the Hub user whose detail for authModuleName carries
// identifier, or nil when no user matches.
//
// Hub cannot filter on the identifier server-side -- its user query understands the
// module but not the detail's identifier -- so this narrows the listing to the module's
// own users and matches over that. The scan is therefore bounded by the size of one
// module's population rather than by the whole user base.
func (c *Client) FindUserByAuthIdentifier(ctx context.Context, authModuleName, identifier string) (*HubUser, error) {
	wanted := strings.TrimSpace(identifier)
	if wanted == "" {
		return nil, fmt.Errorf("identifier must not be empty")
	}

	for skip := 0; ; skip += hubUserDetailPageSize {
		users, err := c.ListUsersByAuthModule(ctx, authModuleName, hubUserDetailPageSize, skip)
		if err != nil {
			return nil, err
		}

		for i := range users {
			if hasAuthIdentifier(users[i], authModuleName, wanted) {
				return &users[i], nil
			}
		}

		if len(users) < hubUserDetailPageSize {
			return nil, nil
		}
	}
}

// hasAuthIdentifier reports whether user carries identifier on a detail belonging to
// authModuleName. The module is checked as well as the identifier because a user has
// one detail per module and their identifiers are unrelated namespaces: matching on
// the value alone could pick up an unrelated provider's id.
func hasAuthIdentifier(user HubUser, authModuleName, identifier string) bool {
	for _, detail := range user.Details {
		if !strings.EqualFold(strings.TrimSpace(detail.AuthModuleName), strings.TrimSpace(authModuleName)) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(detail.Identifier), identifier) {
			return true
		}
	}

	return false
}

// AddUserDetail attaches an authentication detail to a Hub user.
//
// This is what lets an account be reconciled by its identity at an external provider
// before it has ever been used to log in: Hub would otherwise only write the detail on
// first login. When the user does log in later, Hub matches them onto this detail
// instead of provisioning a second account.
func (c *Client) AddUserDetail(ctx context.Context, userID string, detail UserDetail) (*UserDetail, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user id must not be empty")
	}
	if strings.TrimSpace(detail.Type) == "" {
		return nil, fmt.Errorf("user detail type must not be empty")
	}
	if detail.AuthModule == nil || strings.TrimSpace(detail.AuthModule.ID) == "" {
		return nil, fmt.Errorf("user detail auth module id must not be empty")
	}

	rb, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user detail payload: %w", err)
	}

	values := url.Values{}
	values.Set("fields", hubUserDetailOwnFields)

	endpoint := fmt.Sprintf("%s/%s/%s/userdetails?%s", c.HostURL, hubRestUsersAPIPath, url.PathEscape(userID), values.Encode())
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create add user detail request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to add user detail: %w", err)
	}

	var created UserDetail
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created user detail: %w", err)
	}

	return &created, nil
}

// ListUserDetails returns the authentication details attached to a Hub user.
func (c *Client) ListUserDetails(ctx context.Context, userID string) ([]UserDetail, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user id must not be empty")
	}

	values := url.Values{}
	values.Set("fields", hubUserDetailOwnFields)

	endpoint := fmt.Sprintf("%s/%s/%s/userdetails?%s", c.HostURL, hubRestUsersAPIPath, url.PathEscape(userID), values.Encode())
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list user details request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list user details: %w", err)
	}

	var page struct {
		UserDetails []UserDetail `json:"userdetails"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("failed to unmarshal list user details response: %w", err)
	}

	return page.UserDetails, nil
}

// UpdateUserDetailNames writes the descriptive names on an existing user detail.
//
// Hub matches a detail on its identifier alone, so these names change no authentication
// behaviour. They are what makes an identity legible in the Hub UI, which shows a detail
// carrying none of them as a bare external id that no administrator can tie back to a
// person.
//
// This exists because the names cannot be corrected through AddUserDetail: a detail
// whose identifier is already present must not be added a second time, so an account
// linked without names can only be repaired by writing to the detail itself. The
// alternative -- removing the detail and re-adding it -- leaves the account
// unreconcilable in between, which is too high a price for a cosmetic field.
//
// The detail is addressed at the top-level /userdetails/{id}, not under its owning user.
// Hub declares only GET and DELETE on /users/{userId}/userdetails/{detailsId} and answers
// a write there with 405; the top-level path is the one that accepts it.
//
// detailType is the subtype discriminator, such as Oauth2DetailsType, and is required:
// Hub routes the write by it and answers a payload without one with 500
// "UserDetailsCrudService not found by UserDetails of class DetailsJSON". It must match
// the detail's own type -- the $type a read reports.
//
// Either name may be empty, which leaves it as it was: both fields carry omitempty, so an
// empty value sends no key at all. A caller wanting to clear one has no way to express it
// here, and none has needed to.
//
// Hub answers this write with 200 and an empty body, so the result is read back rather
// than parsed from the response.
func (c *Client) UpdateUserDetailNames(ctx context.Context, detailID, detailType, userName, fullName string) (*UserDetail, error) {
	if strings.TrimSpace(detailID) == "" {
		return nil, fmt.Errorf("detail id must not be empty")
	}
	if strings.TrimSpace(detailType) == "" {
		return nil, fmt.Errorf("detail type must not be empty")
	}

	// Only the type and the names are sent. A payload carrying the identifier or the
	// auth module would ask Hub to re-point the detail at a different account identity,
	// which is a different operation from labelling the one that is there.
	rb, err := json.Marshal(UserDetail{Type: detailType, UserName: userName, FullName: fullName})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user detail names payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/%s/%s", c.HostURL, hubRestUserDetailsAPIPath, url.PathEscape(detailID))
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update user detail names request: %w", err)
	}

	if _, err := c.doRequest(req); err != nil {
		return nil, fmt.Errorf("failed to update user detail names: %w", err)
	}

	return c.GetUserDetail(ctx, detailID)
}

// GetUserDetail returns one authentication detail, addressed without its owning user.
func (c *Client) GetUserDetail(ctx context.Context, detailID string) (*UserDetail, error) {
	if strings.TrimSpace(detailID) == "" {
		return nil, fmt.Errorf("detail id must not be empty")
	}

	values := url.Values{}
	values.Set("fields", hubUserDetailOwnFields)

	endpoint := fmt.Sprintf("%s/%s/%s?%s", c.HostURL, hubRestUserDetailsAPIPath, url.PathEscape(detailID), values.Encode())
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get user detail request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user detail: %w", err)
	}

	var detail UserDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user detail: %w", err)
	}

	return &detail, nil
}

// RemoveUserDetail detaches an authentication detail from a Hub user. A detail that is
// already gone is reported as success, so a repeated cleanup settles.
func (c *Client) RemoveUserDetail(ctx context.Context, userID, detailID string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user id must not be empty")
	}
	if strings.TrimSpace(detailID) == "" {
		return fmt.Errorf("detail id must not be empty")
	}

	endpoint := fmt.Sprintf("%s/%s/%s/userdetails/%s", c.HostURL, hubRestUsersAPIPath, url.PathEscape(userID), url.PathEscape(detailID))
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create remove user detail request: %w", err)
	}

	if _, err := c.doRequest(req); err != nil {
		if IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to remove user detail: %w", err)
	}

	return nil
}
