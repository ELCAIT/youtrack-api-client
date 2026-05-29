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
	youtrackUsersAPIPath     = "api/users"
	youtrackGroupsAPIPath    = "api/groups"
	userFieldsQueryParam     = "fields=id,name,login,$type"
	userLifecycleFieldsParam = "fields=id,login,fullName,email,$type"
	groupFieldsQueryParam    = "fields=id,name,$type"
	allUsersGroupFieldsParam = "fields=id,name,allUsersGroup"
	nestedGroupFields        = "fields=id,name,description,ownUsers(id,login),subGroups(id,name),requireTwoFactorAuthentication,viewers(id,name,login,$type),updaters(id,name,login,$type),autoJoin,autoJoinDomain,ringId,icon,allUsersGroup,usersCount,users(id,login)"
	allYoutrackUsers         = pathWithFieldsFormat
	allYoutrackGroups        = pathWithFieldsFormat
	allYoutrackUsersNoFields = "%s/%s"
	specificYoutrackGroup    = "%s/%s/%s?%s"
	specificYoutrackUser     = "%s/%s/%s?%s"
	groupUsersPathFormat     = "%s/%s/%s/users?%s"
	groupUserPathFormat      = "%s/%s/%s/users/%s?%s"
)

func withPagination(fields string, top, skip int) string {
	values := url.Values{}
	values.Set("fields", fields)
	if top > 0 {
		values.Set("$top", strconv.Itoa(top))
	}
	if skip > 0 {
		values.Set("$skip", strconv.Itoa(skip))
	}

	return values.Encode()
}

// ListUsers returns users and supports optional pagination via top/skip.
// Pass 0 for top and skip to use the default server-side pagination.
func (c *Client) ListUsers(ctx context.Context, top, skip int) ([]Holder, error) {
	query := withPagination("id,name,login,$type", top, skip)
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(allYoutrackUsers, c.HostURL, youtrackUsersAPIPath, query), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list users request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	var users []Holder
	err = json.Unmarshal(body, &users)
	if err == nil {
		return users, nil
	}

	var response struct {
		Users []Holder `json:"users"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal list users response: %w", err)
	}

	return response.Users, nil
}

// ListGroups returns groups and supports optional pagination via top/skip.
// Pass 0 for top and skip to use the default server-side pagination.
func (c *Client) ListGroups(ctx context.Context, top, skip int) ([]Holder, error) {
	query := withPagination("id,name,$type", top, skip)
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(allYoutrackGroups, c.HostURL, youtrackGroupsAPIPath, query), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list groups request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	var groups []Holder
	err = json.Unmarshal(body, &groups)
	if err == nil {
		return groups, nil
	}

	var response struct {
		Groups []Holder `json:"usergroups"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal list groups response: %w", err)
	}

	return response.Groups, nil
}

// GetUserByLogin - Returns a user by login (username).
func (c *Client) GetUserByLogin(ctx context.Context, login string) (*Holder, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(allYoutrackUsers, c.HostURL, youtrackUsersAPIPath, userFieldsQueryParam), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get user request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	var users []Holder
	err = json.Unmarshal(body, &users)
	if err != nil {
		var response struct {
			Users []Holder `json:"users"`
		}
		err = json.Unmarshal(body, &response)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal user response: %w", err)
		}
		users = response.Users
	}

	// Filter for exact match on login
	for _, user := range users {
		if user.Login == login {
			return &user, nil
		}
	}

	return nil, fmt.Errorf("user with login '%s' not found", login)
}

// CreateUser creates a new YouTrack user.
func (c *Client) CreateUser(ctx context.Context, user User) (*User, error) {
	rb, err := json.Marshal(user)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(allYoutrackUsers, c.HostURL, youtrackUsersAPIPath, userLifecycleFieldsParam), bytes.NewReader(rb))
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

// UpdateUser updates an existing YouTrack user by ID.
func (c *Client) UpdateUser(ctx context.Context, userID string, user User) (*User, error) {
	rb, err := json.Marshal(user)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update user payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(specificYoutrackUser, c.HostURL, youtrackUsersAPIPath, userID, userLifecycleFieldsParam), bytes.NewReader(rb))
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

// DeleteUser deletes a YouTrack user by ID.
func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	url := fmt.Sprintf(allYoutrackUsersNoFields+"/%s", c.HostURL, youtrackUsersAPIPath, userID)
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, url, nil)
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

// GetUserGroupByName - Returns a user group by name.
func (c *Client) GetUserGroupByName(ctx context.Context, name string) (*Holder, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(allYoutrackGroups, c.HostURL, youtrackGroupsAPIPath, groupFieldsQueryParam), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get user group request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user groups: %w", err)
	}

	var groups []Holder
	err = json.Unmarshal(body, &groups)
	if err != nil {
		var response struct {
			Groups []Holder `json:"usergroups"`
		}
		err = json.Unmarshal(body, &response)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal user group response: %w", err)
		}
		groups = response.Groups
	}

	// Filter for exact match on name (case-insensitive)
	for _, group := range groups {
		if strings.EqualFold(group.Name, name) {
			return &group, nil
		}
	}

	return nil, fmt.Errorf("user group with name '%s' not found", name)
}

// CreateGroup creates a new YouTrack group.
func (c *Client) CreateGroup(ctx context.Context, group NestedGroup) (*NestedGroup, error) {
	rb, err := json.Marshal(group)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal group payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(allYoutrackGroups, c.HostURL, youtrackGroupsAPIPath, nestedGroupFields), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create create group request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	var created NestedGroup
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created group: %w", err)
	}

	return &created, nil
}

// GetGroupByID returns a YouTrack group by ID.
func (c *Client) GetGroupByID(ctx context.Context, groupID string) (*NestedGroup, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(specificYoutrackGroup, c.HostURL, youtrackGroupsAPIPath, groupID, nestedGroupFields), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get group request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	var group NestedGroup
	if err := json.Unmarshal(body, &group); err != nil {
		return nil, fmt.Errorf("failed to unmarshal group: %w", err)
	}

	return &group, nil
}

// UpdateGroup updates an existing YouTrack group by ID.
func (c *Client) UpdateGroup(ctx context.Context, groupID string, group NestedGroup) (*NestedGroup, error) {
	rb, err := json.Marshal(group)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update group payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(specificYoutrackGroup, c.HostURL, youtrackGroupsAPIPath, groupID, nestedGroupFields), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update group request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update group: %w", err)
	}

	var updated NestedGroup
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated group: %w", err)
	}

	return &updated, nil
}

// GetAllUsersGroup returns the built-in "All Users" group which is used as a
// required successor when deleting any other group.
func (c *Client) GetAllUsersGroup(ctx context.Context) (*NestedGroup, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(allYoutrackGroups, c.HostURL, youtrackGroupsAPIPath, allUsersGroupFieldsParam), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get all-users group request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get groups: %w", err)
	}

	var groups []NestedGroup
	if err := json.Unmarshal(body, &groups); err != nil {
		return nil, fmt.Errorf("failed to unmarshal groups: %w", err)
	}

	for i := range groups {
		if groups[i].AllUsersGroup {
			return &groups[i], nil
		}
	}

	return nil, fmt.Errorf("all-users group not found")
}

// DeleteGroup deletes a YouTrack group by ID. The successorID is the ID of
// the group that will receive the users of the deleted group; the YouTrack API
// requires this parameter.
func (c *Client) DeleteGroup(ctx context.Context, groupID, successorID string) error {
	url := fmt.Sprintf("%s/%s/%s?successor=%s", c.HostURL, youtrackGroupsAPIPath, groupID, successorID)
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete group request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		if IsNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to delete group: %w", err)
	}

	return nil
}

// AddUserToGroup adds a user to a YouTrack group by IDs.
func (c *Client) AddUserToGroup(ctx context.Context, groupID, userID string) error {
	payload := Holder{Id: userID}
	rb, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal add user to group payload: %w", err)
	}

	url := fmt.Sprintf(groupUsersPathFormat, c.HostURL, youtrackGroupsAPIPath, groupID, userLifecycleFieldsParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, url, bytes.NewReader(rb))
	if err != nil {
		return fmt.Errorf("failed to create add user to group request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to add user to group: %w", err)
	}

	return nil
}

// RemoveUserFromGroup removes a user from a YouTrack group by IDs.
func (c *Client) RemoveUserFromGroup(ctx context.Context, groupID, userID string) error {
	url := fmt.Sprintf(groupUserPathFormat, c.HostURL, youtrackGroupsAPIPath, groupID, userID, userLifecycleFieldsParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create remove user from group request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		if IsNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to remove user from group: %w", err)
	}

	return nil
}
