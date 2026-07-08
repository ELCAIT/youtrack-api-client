package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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

// UpdateUser updates a user using Hub-style lifecycle semantics.
func (c *Client) UpdateUser(ctx context.Context, userID string, user User) (*User, error) {
	// #nosec G117 -- password is intentionally sent to the Hub API when updating users.
	rb, err := json.Marshal(user)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update user payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(hubSpecificUserPath, c.HostURL, youtrackUsersAPIPath, userID, hubUserLifecycleFields), bytes.NewReader(rb))
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

// BanUser bans a user using Hub-style lifecycle semantics.
func (c *Client) BanUser(ctx context.Context, userID string) (*User, error) {
	updated, err := c.UpdateUser(ctx, userID, User{Banned: true})
	if err != nil {
		return nil, fmt.Errorf("failed to ban user: %w", err)
	}

	return updated, nil
}

// DeleteUser deletes a user and passes a successor as required by Hub semantics.
func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	guest, err := c.GetUserByLogin(ctx, "guest")
	if err != nil {
		return fmt.Errorf("failed to resolve delete user successor: %w", err)
	}

	endpoint := fmt.Sprintf(hubAllUsersNoFields+"/%s?successor=%s", c.HostURL, youtrackUsersAPIPath, userID, url.QueryEscape(guest.Id))
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

	return nil
}

// AddUserToGroup adds a user to a group using Hub usergroups endpoints.
func (c *Client) AddUserToGroup(ctx context.Context, groupID, userID string) error {
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
			endpoint: fmt.Sprintf(hubGroupUsersNoFields, c.HostURL, hubUserGroupsAPIPath, groupID),
			body:     userRB,
		},
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUsersNoFields, c.HostURL, hubRestUserGroupsAPIPath, groupID),
			body:     userRB,
		},
		{
			method:   http.MethodPut,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubUserGroupsAPIPath, groupID, userID),
		},
		{
			method:   http.MethodPut,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubRestUserGroupsAPIPath, groupID, userID),
		},
		// Compatibility variants with explicit fields query.
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUsersPathFormat, c.HostURL, hubUserGroupsAPIPath, groupID, hubUserLifecycleFields),
			body:     userRB,
		},
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUsersPathFormat, c.HostURL, hubRestUserGroupsAPIPath, groupID, hubUserLifecycleFields),
			body:     userRB,
		},
		// Legacy/compatibility endpoints for older setups.
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUsersPathFormat, c.HostURL, youtrackGroupsAPIPath, groupID, hubUserLifecycleFields),
			body:     userRB,
		},
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubUserGroupsAPIPath, groupID, userID),
			body:     userRB,
		},
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubRestUserGroupsAPIPath, groupID, userID),
			body:     userRB,
		},
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, youtrackGroupsAPIPath, groupID, userID),
			body:     userRB,
		},
		{
			method:   http.MethodPut,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubUserGroupsAPIPath, groupID, userID),
		},
		{
			method:   http.MethodPut,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubRestUserGroupsAPIPath, groupID, userID),
		},
		{
			method:   http.MethodPut,
			endpoint: fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, youtrackGroupsAPIPath, groupID, userID),
		},
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubUserGroupPathFormat, c.HostURL, youtrackUsersAPIPath, userID, groupID),
			body:     groupRB,
		},
		{
			method:   httpMethodPost,
			endpoint: fmt.Sprintf(hubUserGroupPathFormat, c.HostURL, hubRestUsersAPIPath, userID, groupID),
			body:     groupRB,
		},
		{
			method:   http.MethodPut,
			endpoint: fmt.Sprintf(hubUserGroupPathFormat, c.HostURL, youtrackUsersAPIPath, userID, groupID),
		},
		{
			method:   http.MethodPut,
			endpoint: fmt.Sprintf(hubUserGroupPathFormat, c.HostURL, hubRestUsersAPIPath, userID, groupID),
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
func (c *Client) RemoveUserFromGroup(ctx context.Context, groupID, userID string) error {
	deleteAt := func(endpoint string) error {
		req, reqErr := http.NewRequestWithContext(ctx, httpMethodDelete, endpoint, nil)
		if reqErr != nil {
			return reqErr
		}
		_, reqErr = c.doRequest(req)
		return reqErr
	}

	attempts := []string{
		fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubUserGroupsAPIPath, groupID, userID),
		fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, youtrackGroupsAPIPath, groupID, userID),
		fmt.Sprintf(hubUserGroupPathFormat, c.HostURL, youtrackUsersAPIPath, userID, groupID),
		fmt.Sprintf(hubGroupUserPathFormat, c.HostURL, hubRestUserGroupsAPIPath, groupID, userID),
		fmt.Sprintf(hubUserGroupPathFormat, c.HostURL, hubRestUsersAPIPath, userID, groupID),
	}

	for _, endpoint := range attempts {
		err := deleteAt(endpoint)
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
