package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	hubUserGroupsAPIPath     = "api/usergroups"
	hubRestUserGroupsAPIPath = "hub/api/rest/usergroups"
	hubRestUsersAPIPath      = "hub/api/rest/users"
	hubUserLifecycleFields   = "fields=id,ringId,login,fullName,email,banned,$type"
	hubAllUsersNoFields      = "%s/%s"
	hubSpecificUserPath      = "%s/%s/%s?%s"
	hubGroupUsersPathFormat  = "%s/%s/%s/users?%s"
	hubGroupUsersNoFields    = "%s/%s/%s/users"
	hubGroupUserPathFormat   = "%s/%s/%s/users/%s"
	hubUserGroupPathFormat   = "%s/%s/%s/groups/%s"
	ringIDFields             = "fields=ringId"
	groupOwnUsersFields      = "fields=ownUsers(id,ringId)"
)

func isRetryableMembershipEndpointError(err error) bool {
	if err == nil {
		return false
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		return false
	}

	return httpErr.StatusCode == http.StatusNotFound || httpErr.StatusCode == http.StatusMethodNotAllowed
}

// isYouTrackEntityID reports whether id has the shape of a YouTrack database id,
// "<type>-<number>" such as "4-241", as opposed to a Hub id, which is a UUID.
func isYouTrackEntityID(id string) bool {
	typ, num, found := strings.Cut(id, "-")
	return found && isDigits(typ) && isDigits(num)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// resolveHubID returns the Hub id of a YouTrack user or group, read from its ringId.
//
// Hub's membership endpoints only know Hub ids: from YouTrack 2026.3, a YouTrack id
// such as the one CreateGroup returns is answered with 404 or 405 on every endpoint
// variant. An id that is not in YouTrack's format is taken to be a Hub id already and
// returned unchanged, and so is one YouTrack does not know or reports without a ringId,
// leaving the membership endpoints to decide.
func (c *Client) resolveHubID(ctx context.Context, apiPath, id string) (string, error) {
	if !isYouTrackEntityID(id) {
		return id, nil
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(hubSpecificUserPath, c.HostURL, apiPath, url.PathEscape(id), ringIDFields), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create resolve hub id request: %w", err)
	}

	body, err := c.doRequest(req)
	if isRetryableMembershipEndpointError(err) {
		return id, nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to resolve hub id of %q: %w", id, err)
	}

	var entity struct {
		RingID string `json:"ringId"`
	}
	if err := json.Unmarshal(body, &entity); err != nil {
		return "", fmt.Errorf("failed to unmarshal ringId of %q: %w", id, err)
	}
	if entity.RingID == "" {
		return id, nil
	}

	return entity.RingID, nil
}

// resolveMembershipIDs returns the Hub ids of a group and a user for a membership call.
func (c *Client) resolveMembershipIDs(ctx context.Context, groupID, userID string) (string, string, error) {
	hubGroupID, err := c.resolveHubID(ctx, youtrackGroupsAPIPath, groupID)
	if err != nil {
		return "", "", err
	}

	hubUserID, err := c.resolveHubID(ctx, youtrackUsersAPIPath, userID)
	if err != nil {
		return "", "", err
	}

	return hubGroupID, hubUserID, nil
}

func (c *Client) sendMembershipRequest(ctx context.Context, method, endpoint string, body []byte) error {
	reader := bytes.NewReader(body)

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req)
	return err
}

// CreateUser creates a user using Hub-style lifecycle semantics.
func (c *Client) CreateUser(ctx context.Context, user User) (*User, error) {
	// #nosec G117 -- password is intentionally sent to the Hub API when creating users.
	rb, err := json.Marshal(user)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(allYoutrackUsers, c.HostURL, youtrackUsersAPIPath, hubUserLifecycleFields), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create create user request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	var created User
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created user: %w", err)
	}

	return &created, nil
}

// GetUserByID returns a single user, including the lifecycle fields (email, banned)
// that the paginated user listing does not carry.
func (c *Client) GetUserByID(ctx context.Context, userID string) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(hubSpecificUserPath, c.HostURL, youtrackUsersAPIPath, url.PathEscape(userID), hubUserLifecycleFields), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get user request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	return &user, nil
}

// UpdateUser updates a user using Hub-style lifecycle semantics.
func (c *Client) UpdateUser(ctx context.Context, userID string, user User) (*User, error) {
	// #nosec G117 -- password is intentionally sent to the Hub API when updating users.
	rb, err := json.Marshal(user)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update user payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(hubSpecificUserPath, c.HostURL, youtrackUsersAPIPath, url.PathEscape(userID), hubUserLifecycleFields), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update user request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	var updated User
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated user: %w", err)
	}

	return &updated, nil
}

// bannedPayload carries the banned flag on its own, without omitempty, which is what
// lets an unban reach the wire at all: User.Banned is omitempty, so a false there is
// dropped from the JSON and Hub is asked to change nothing.
type bannedPayload struct {
	Banned bool `json:"banned"`
}

// SetUserBanned bans or unbans a user using Hub-style lifecycle semantics.
//
// Hub has no separate unban endpoint -- both directions are a write of the account's
// banned flag -- so this is the single call for either, and the only one that can
// express banned=false.
func (c *Client) SetUserBanned(ctx context.Context, userID string, banned bool) (*User, error) {
	rb, err := json.Marshal(bannedPayload{Banned: banned})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal set user banned payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(hubSpecificUserPath, c.HostURL, youtrackUsersAPIPath, url.PathEscape(userID), hubUserLifecycleFields), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create set user banned request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to set user banned: %w", err)
	}

	var updated User
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, fmt.Errorf("failed to unmarshal set user banned response: %w", err)
	}

	return &updated, nil
}

// BanUser bans a user using Hub-style lifecycle semantics.
//
// It is SetUserBanned(ctx, userID, true); use SetUserBanned directly to unban.
func (c *Client) BanUser(ctx context.Context, userID string) (*User, error) {
	updated, err := c.SetUserBanned(ctx, userID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to ban user: %w", err)
	}

	return updated, nil
}

// UnbanUser lifts a ban using Hub-style lifecycle semantics.
func (c *Client) UnbanUser(ctx context.Context, userID string) (*User, error) {
	updated, err := c.SetUserBanned(ctx, userID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to unban user: %w", err)
	}

	return updated, nil
}

// DeleteUser deletes a user and passes a successor as required by Hub semantics.
func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	guest, err := c.GetUserByLogin(ctx, "guest")
	if err != nil {
		return fmt.Errorf("failed to resolve delete user successor: %w", err)
	}

	endpoint := fmt.Sprintf(hubAllUsersNoFields+"/%s?successor=%s", c.HostURL, youtrackUsersAPIPath, url.PathEscape(userID), url.QueryEscape(guest.Id))
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete user request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		if IsNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Hub acknowledges the delete before applying it, so the user stays readable
	// for a moment afterwards; wait until it is gone so a read-back reports 404.
	return awaitAsyncProcessing(ctx, func(attemptCtx context.Context) bool {
		_, getErr := c.GetUserByID(attemptCtx, userID)
		return IsNotFoundError(getErr)
	})
}

// AddUserToGroup adds a user to a group using Hub usergroups endpoints.
//
// groupID and userID may be Hub ids or YouTrack ids; a YouTrack id is resolved to the
// Hub id (its ringId) first, since the Hub endpoints do not accept it.
//
// When groupID is a YouTrack id, it also waits for YouTrack to report the user as a
// member: YouTrack picks up a Hub membership change asynchronously, so a group read
// issued right after the write would otherwise still show the previous members.
func (c *Client) AddUserToGroup(ctx context.Context, groupID, userID string) error {
	hubGroupID, hubUserID, err := c.resolveMembershipIDs(ctx, groupID, userID)
	if err != nil {
		return fmt.Errorf("failed to add user to group: %w", err)
	}

	if err := c.addHubMembership(ctx, hubGroupID, hubUserID); err != nil {
		return err
	}

	return c.awaitYouTrackMembership(ctx, groupID, userID, hubUserID, true)
}

// addHubMembership adds a user to a group, both given by their Hub id, trying each
// known endpoint variant in turn.
func (c *Client) addHubMembership(ctx context.Context, groupID, userID string) error {
	userPayload := Holder{Id: userID}
	userRB, err := json.Marshal(userPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal add user to group payload: %w", err)
	}

	groupPayload := Holder{Id: groupID}
	groupRB, err := json.Marshal(groupPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal add group to user payload: %w", err)
	}

	type membershipAttempt struct {
		method   string
		endpoint string
		body     []byte
	}

	attempts := []membershipAttempt{
		// Canonical Hub usergroup membership endpoints (preferred).
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUsersNoFields, c.HostURL, hubUserGroupsAPIPath, url.PathEscape(groupID)),
			body:     userRB,
		},
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUsersNoFields, c.HostURL, hubRestUserGroupsAPIPath, url.PathEscape(groupID)),
			body:     userRB,
		},
		{
			method:   http.MethodPut,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubUserGroupsAPIPath, url.PathEscape(groupID), url.PathEscape(userID)),
		},
		{
			method:   http.MethodPut,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubRestUserGroupsAPIPath, url.PathEscape(groupID), url.PathEscape(userID)),
		},
		// Compatibility variants with explicit fields query.
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUsersPathFormat, c.HostURL, hubUserGroupsAPIPath, url.PathEscape(groupID), hubUserLifecycleFields),
			body:     userRB,
		},
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUsersPathFormat, c.HostURL, hubRestUserGroupsAPIPath, url.PathEscape(groupID), hubUserLifecycleFields),
			body:     userRB,
		},
		// Legacy/compatibility endpoints for older setups.
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUsersPathFormat, c.HostURL, youtrackGroupsAPIPath, url.PathEscape(groupID), hubUserLifecycleFields),
			body:     userRB,
		},
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubUserGroupsAPIPath, url.PathEscape(groupID), url.PathEscape(userID)),
			body:     userRB,
		},
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubRestUserGroupsAPIPath, url.PathEscape(groupID), url.PathEscape(userID)),
			body:     userRB,
		},
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, youtrackGroupsAPIPath, url.PathEscape(groupID), url.PathEscape(userID)),
			body:     userRB,
		},
		{
			method:   http.MethodPut,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubUserGroupsAPIPath, url.PathEscape(groupID), url.PathEscape(userID)),
		},
		{
			method:   http.MethodPut,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubRestUserGroupsAPIPath, url.PathEscape(groupID), url.PathEscape(userID)),
		},
		{
			method:   http.MethodPut,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, youtrackGroupsAPIPath, url.PathEscape(groupID), url.PathEscape(userID)),
		},
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubUserGroupPathFormat, c.HostURL, youtrackUsersAPIPath, url.PathEscape(userID), url.PathEscape(groupID)),
			body:     groupRB,
		},
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubUserGroupPathFormat, c.HostURL, hubRestUsersAPIPath, url.PathEscape(userID), url.PathEscape(groupID)),
			body:     groupRB,
		},
		{
			method:   http.MethodPut,
			endpoint: fmt.Sprintf(hubUserGroupPathFormat, c.HostURL, youtrackUsersAPIPath, url.PathEscape(userID), url.PathEscape(groupID)),
		},
		{
			method:   http.MethodPut,
			endpoint: fmt.Sprintf(hubUserGroupPathFormat, c.HostURL, hubRestUsersAPIPath, url.PathEscape(userID), url.PathEscape(groupID)),
		},
	}

	var lastErr error
	for _, attempt := range attempts {
		err := c.sendMembershipRequest(ctx, attempt.method, attempt.endpoint, attempt.body)
		if err == nil {
			return nil
		}
		if isRetryableMembershipEndpointError(err) {
			lastErr = err
			continue
		}
		return fmt.Errorf("failed to add user to group: %w", err)
	}

	return fmt.Errorf("failed to add user to group: %w", lastErr)
}

// RemoveUserFromGroup removes a user from a group using Hub usergroups endpoints.
//
// groupID and userID may be Hub ids or YouTrack ids, and a YouTrack groupID is read
// back until the user is gone, as for AddUserToGroup.
func (c *Client) RemoveUserFromGroup(ctx context.Context, groupID, userID string) error {
	hubGroupID, hubUserID, err := c.resolveMembershipIDs(ctx, groupID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove user from group: %w", err)
	}

	if err := c.removeHubMembership(ctx, hubGroupID, hubUserID); err != nil {
		return err
	}

	return c.awaitYouTrackMembership(ctx, groupID, userID, hubUserID, false)
}

// removeHubMembership removes a user from a group, both given by their Hub id, trying
// each known endpoint variant in turn.
func (c *Client) removeHubMembership(ctx context.Context, groupID, userID string) error {
	attempts := []string{
		fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubUserGroupsAPIPath, url.PathEscape(groupID), url.PathEscape(userID)),
		fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, youtrackGroupsAPIPath, url.PathEscape(groupID), url.PathEscape(userID)),
		fmt.Sprintf(hubUserGroupPathFormat, c.HostURL, youtrackUsersAPIPath, url.PathEscape(userID), url.PathEscape(groupID)),
		fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubRestUserGroupsAPIPath, url.PathEscape(groupID), url.PathEscape(userID)),
		fmt.Sprintf(hubUserGroupPathFormat, c.HostURL, hubRestUsersAPIPath, url.PathEscape(userID), url.PathEscape(groupID)),
	}

	for _, endpoint := range attempts {
		err := c.sendMembershipRequest(ctx, httpMethodDelete, endpoint, nil)
		if err == nil {
			return nil
		}
		if isRetryableMembershipEndpointError(err) || IsNotFoundError(err) {
			continue
		}
		return fmt.Errorf("failed to remove user from group: %w", err)
	}

	// Remove is idempotent: if all known endpoint variants returned 404/405,
	// the membership is effectively absent.
	return nil
}

// awaitYouTrackMembership waits, when groupID is a YouTrack id, until YouTrack reports
// whether the user is a member of the group as wantMember says. The user is matched by
// the id the caller gave or by its Hub id, whichever kind the caller used.
//
// A failed read ends the wait without an error: the membership write was already
// acknowledged by Hub, which is what the caller asked for.
func (c *Client) awaitYouTrackMembership(ctx context.Context, groupID, userID, hubUserID string, wantMember bool) error {
	if !isYouTrackEntityID(groupID) {
		return nil
	}

	err := awaitAsyncProcessing(ctx, func(attemptCtx context.Context) bool {
		isMember, readErr := c.isYouTrackGroupMember(attemptCtx, groupID, userID, hubUserID)
		return readErr != nil || isMember == wantMember
	})
	if err != nil {
		return fmt.Errorf("failed to wait for group membership to be applied: %w", err)
	}

	return nil
}

// isYouTrackGroupMember reports whether YouTrack lists the user among the group's own
// users, matching it by YouTrack id or by Hub id.
func (c *Client) isYouTrackGroupMember(ctx context.Context, groupID, userID, hubUserID string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(specificYoutrackGroup, c.HostURL, youtrackGroupsAPIPath, url.PathEscape(groupID), groupOwnUsersFields), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create get group members request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return false, fmt.Errorf("failed to get group members: %w", err)
	}

	var group struct {
		OwnUsers []struct {
			ID     string `json:"id"`
			RingID string `json:"ringId"`
		} `json:"ownUsers"`
	}
	if err := json.Unmarshal(body, &group); err != nil {
		return false, fmt.Errorf("failed to unmarshal group members: %w", err)
	}

	for _, u := range group.OwnUsers {
		if u.ID == userID || u.RingID == hubUserID {
			return true, nil
		}
	}

	return false, nil
}
