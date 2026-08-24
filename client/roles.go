package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	permissionGraphFieldsParam     = "fields=id,name,key,impliedPermissions(id,name,key),dependentPermissions(id,name,key)"
	specificYoutrackRole           = "%s/%s/%s?%s"
	roleFields                     = "id,key,name,description,permissions(id,key,name)"
	roleFieldsQueryParam           = "fields=" + roleFields
	youtrackRolePermByIDAPIPath    = "%s/api/roles/%s/permissions/%s"

	// roleLookupPageSize is the page size used when paging through roles to find
	// a match by name.
	roleLookupPageSize = 100

	errMarshalRole = "failed to marshal role: %w"
)

// errRoleNotFound is returned by GetYoutrackRoleByName when no role matches the name.
var errRoleNotFound = fmt.Errorf("role %w", ErrNotFound)

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

// GetPermissionGraph fetches permissions with implied/dependent relations from the YouTrack REST API.
func (c *Client) GetPermissionGraph(ctx context.Context) ([]PermissionGraphEntry, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet,
		fmt.Sprintf(pathWithFieldsFormat, c.HostURL, youtrackPermissionsAPIPath, permissionGraphFieldsParam), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get permission graph request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get permission graph: %w", err)
	}

	var graph []PermissionGraphEntry
	if err = json.Unmarshal(body, &graph); err != nil {
		return nil, fmt.Errorf("failed to unmarshal permission graph response: %w", err)
	}

	return graph, nil
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

// ListYoutrackRoles returns the roles defined on the instance and supports optional
// pagination via top/skip. Pass 0 for top and skip to use the default server-side
// pagination.
func (c *Client) ListYoutrackRoles(ctx context.Context, top, skip int) ([]Role, error) {
	query := withPagination(roleFields, top, skip)
	endpoint := fmt.Sprintf(pathWithFieldsFormat, c.HostURL, youtrackRolesAPIPath, query)

	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list roles request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	var roles []Role
	if err = json.Unmarshal(body, &roles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal roles response: %w", err)
	}

	return roles, nil
}

// IsRoleNotFoundError checks whether an error indicates that a role could not be found by name.
// Use IsNotFound instead when the entity type does not matter.
func IsRoleNotFoundError(err error) bool {
	return errors.Is(err, errRoleNotFound)
}

// GetYoutrackRoleByName retrieves a role by its name, paging through all roles. An
// exact match wins over a case-insensitive one on any page. It returns an error
// wrapping ErrNotFound when no role matches; test for it with IsNotFound.
//
// Role names are what configuration and documentation refer to ("ELCA Reader"),
// while assignment APIs need the role ID, so callers typically resolve names once
// at startup and cache the result rather than looking them up per operation.
func (c *Client) GetYoutrackRoleByName(ctx context.Context, name string) (*Role, error) {
	role, err := lookupByNamePaginated(ctx, roleLookupPageSize, name, c.getRolePage, roleName)
	if err != nil {
		return nil, err
	}
	if role != nil {
		return role, nil
	}

	return nil, entityNotFoundf(errRoleNotFound, "role with name %q", name)
}

func (c *Client) getRolePage(ctx context.Context, skip int) ([]Role, error) {
	return c.ListYoutrackRoles(ctx, roleLookupPageSize, skip)
}

func roleName(role Role) string {
	return role.Name
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

	// Wait for the API to process the change (async processing)
	waitForAsyncProcessing()

	return c.GetYoutrackRoleById(ctx, role.Id)
}

// DeleteYoutrackRole deletes a role via the YouTrack API.
func (c *Client) DeleteYoutrackRole(ctx context.Context, roleId string) error {
	return deleteByID(ctx, c, roleId, deleteConfig{
		HostURL:   c.HostURL,
		APIPath:   youtrackRolesAPIPath,
		ErrCreate: "failed to create delete role request: %w",
		ErrFetch:  "failed to delete role: %w",
	})
}
