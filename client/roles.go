package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// API endpoint path templates
const (
	apiBasePath                    = "hub/api/rest"
	roleByIDPath                   = "%s/%s/roles/%s"
	rolePermissionByIDPath         = "%s/%s/roles/%s/permissions/%s"
	permissionsPath                = "%s/%s/permissions"
	youtrackPermissionsAPIPath     = "api/permissions"
	youtrackPermissionsFieldsParam = "fields=id,name,key"
	specificYoutrackRole           = "%s/%s/%s?%s"
	roleFieldsQueryParam           = "fields=id,key,name,description,permissions(id,key,name)"
	youtrackRolePermByIDAPIPath    = "%s/api/roles/%s/permissions/%s"

	errMarshalRole = "failed to marshal role: %w"
)

// GetAllPermissions returns a merged permission list from the YouTrack API (primary) and Hub API.
// YouTrack key-style IDs take precedence; Hub-only entries are appended.
func (c *Client) GetAllPermissions(ctx context.Context) ([]Permission, error) {
	ytPerms, err := c.getAllYoutrackPermissions(ctx)
	if err != nil {
		return nil, err
	}

	hubPerms, err := c.getAllHubPermissions(ctx)
	if err != nil {
		return nil, err
	}

	return mergePermissionLists(ytPerms, hubPerms), nil
}

// getAllHubPermissions fetches permissions from the Hub REST API.
func (c *Client) getAllHubPermissions(ctx context.Context) ([]Permission, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(permissionsPath, c.HostURL, apiBasePath), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get hub permissions request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get hub permissions: %w", err)
	}

	var response PermissionsResponse
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal hub permissions response: %w", err)
	}

	return response.Permissions, nil
}

// getAllYoutrackPermissions fetches permissions from the YouTrack REST API.
func (c *Client) getAllYoutrackPermissions(ctx context.Context) ([]Permission, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet,
		fmt.Sprintf(pathWithFieldsFormat, c.HostURL, youtrackPermissionsAPIPath, youtrackPermissionsFieldsParam), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get youtrack permissions request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get youtrack permissions: %w", err)
	}

	var perms []Permission
	if err = json.Unmarshal(body, &perms); err != nil {
		return nil, fmt.Errorf("failed to unmarshal youtrack permissions response: %w", err)
	}

	return perms, nil
}

// mergePermissionLists deduplicates two permission slices by name; primary takes precedence.
func mergePermissionLists(primary, secondary []Permission) []Permission {
	seen := make(map[string]bool, len(primary))
	result := make([]Permission, 0, len(primary)+len(secondary))

	for _, p := range primary {
		seen[strings.ToLower(p.Name)] = true
		result = append(result, p)
	}

	for _, p := range secondary {
		if !seen[strings.ToLower(p.Name)] {
			seen[strings.ToLower(p.Name)] = true
			result = append(result, p)
		}
	}

	return result
}

// GetYoutrackRoleById returns a YouTrack role by ID.
func (c *Client) GetYoutrackRoleById(ctx context.Context, roleId string) (*Role, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(specificYoutrackRole, c.HostURL, youtrackRolesAPIPath, roleId, roleFieldsQueryParam), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get YouTrack role request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get YouTrack role: %w", err)
	}

	var role Role
	if err = json.Unmarshal(body, &role); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YouTrack role: %w", err)
	}

	return &role, nil
}

// CreateYoutrackRole creates a role via the YouTrack API, including permissions.
func (c *Client) CreateYoutrackRole(ctx context.Context, role Role) (*Role, error) {
	payload := Role{
		Key:         role.Key,
		Name:        role.Name,
		Description: role.Description,
		Permissions: role.Permissions,
	}

	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(errMarshalRole, err)
	}

	url := fmt.Sprintf(pathWithFieldsFormat, c.HostURL, youtrackRolesAPIPath, roleFieldsQueryParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, url, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create role request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	var created Role
	if err = json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created role: %w", err)
	}

	return &created, nil
}

// UpdateYoutrackRole updates name, description, and permissions via the YouTrack API.
// The key is immutable. Permissions must use key-style IDs, not Hub UUIDs.
func (c *Client) UpdateYoutrackRole(ctx context.Context, role Role) (*Role, error) {
	payload := Role{
		Name:        role.Name,
		Description: role.Description,
		Permissions: role.Permissions,
	}

	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(errMarshalRole, err)
	}

	url := fmt.Sprintf(specificYoutrackRole, c.HostURL, youtrackRolesAPIPath, role.Id, roleFieldsQueryParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, url, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update role request: %w", err)
	}

	if _, err = c.doRequest(req); err != nil {
		return nil, fmt.Errorf("failed to update role: %w", err)
	}

	return c.GetYoutrackRoleById(ctx, role.Id)
}

// DeleteYoutrackRole deletes a role via the YouTrack API.
func (c *Client) DeleteYoutrackRole(ctx context.Context, roleId string) error {
	url := fmt.Sprintf("%s/%s/%s", c.HostURL, youtrackRolesAPIPath, roleId)
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete role request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil && !IsNotFoundError(err) {
		return fmt.Errorf("failed to delete role: %w", err)
	}

	return nil
}
