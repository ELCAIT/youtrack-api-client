package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const (
	assignedRolesAPIPath    = "api/assignedRoles"
	youtrackRolesAPIPath    = "api/roles"
	assignedRoleFieldsParam = "fields=id,role(id,name,description),scope(id,$type,project(id,name,shortName)),holder(id,name,login,$type),$type"
	pathWithFieldsFormat    = "%s/%s?%s"
	allAssignedRoles        = pathWithFieldsFormat
	specificAssignedRole    = "%s/%s/%s?%s"
	assignedRolesByQuery    = "%s/%s?%s&query=%s"

	assignedRoleType = "AssignedRole"
)

// listAssignedRoles fetches assigned roles from the given endpoint. The YouTrack
// REST API returns a bare JSON array for this resource, not a wrapped object.
func (c *Client) listAssignedRoles(ctx context.Context, endpoint string) ([]AssignedRole, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list assigned roles request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list assigned roles: %w", err)
	}

	var roles []AssignedRole
	if err := json.Unmarshal(body, &roles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal assigned roles response: %w", err)
	}

	return roles, nil
}

// GetAllAssignedRoles - Returns list of assigned roles.
func (c *Client) GetAllAssignedRoles(ctx context.Context) ([]AssignedRole, error) {
	endpoint := fmt.Sprintf(allAssignedRoles, c.HostURL, assignedRolesAPIPath, assignedRoleFieldsParam)
	return c.listAssignedRoles(ctx, endpoint)
}

// GetAssignedRolesByHolder - Returns the role assignments held by a specific user
// or group, identified by holder ID.
func (c *Client) GetAssignedRolesByHolder(ctx context.Context, holderID string) ([]AssignedRole, error) {
	endpoint := fmt.Sprintf(assignedRolesByQuery, c.HostURL, assignedRolesAPIPath, assignedRoleFieldsParam, url.QueryEscape("holder:"+holderID))
	return c.listAssignedRoles(ctx, endpoint)
}

// GetAssignedRoleById - Returns a specific assigned role by ID.
func (c *Client) GetAssignedRoleById(ctx context.Context, roleAssignmentId string) (*AssignedRole, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(specificAssignedRole, c.HostURL, assignedRolesAPIPath, roleAssignmentId, assignedRoleFieldsParam), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get assigned role request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get assigned role: %w", err)
	}

	var assignedRole AssignedRole
	err = json.Unmarshal(body, &assignedRole)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal assigned role response: %w", err)
	}

	return &assignedRole, nil
}

// CreateAssignedRole - Creates a new role assignment. Set Scope.Type to "GlobalScope",
// "OrganizationScope", or "ProjectScope" to control where the role applies; for
// "ProjectScope", also set Scope.Project to the target project.
func (c *Client) CreateAssignedRole(ctx context.Context, assignedRole AssignedRole) (*AssignedRole, error) {
	assignedRole.Type = assignedRoleType

	rb, err := json.Marshal(assignedRole)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal assigned role: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(allAssignedRoles, c.HostURL, assignedRolesAPIPath, assignedRoleFieldsParam), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create assigned role request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create assigned role with payload %s: %w", string(rb), err)
	}

	var createdRole AssignedRole
	err = json.Unmarshal(body, &createdRole)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal created assigned role: %w", err)
	}

	return &createdRole, nil
}

// UpdateAssignedRole - Updates an existing role assignment.
func (c *Client) UpdateAssignedRole(ctx context.Context, roleAssignmentId string, assignedRole AssignedRole) (*AssignedRole, error) {
	assignedRole.Type = assignedRoleType

	rb, err := json.Marshal(assignedRole)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal assigned role: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(specificAssignedRole, c.HostURL, assignedRolesAPIPath, roleAssignmentId, assignedRoleFieldsParam), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update assigned role request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update assigned role with payload %s: %w", string(rb), err)
	}

	var updatedRole AssignedRole
	err = json.Unmarshal(body, &updatedRole)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated assigned role: %w", err)
	}

	return &updatedRole, nil
}

// DeleteAssignedRole - Deletes a role assignment.
func (c *Client) DeleteAssignedRole(ctx context.Context, roleAssignmentId string) error {
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, fmt.Sprintf(specificAssignedRole, c.HostURL, assignedRolesAPIPath, roleAssignmentId, assignedRoleFieldsParam), nil)
	if err != nil {
		return fmt.Errorf("failed to create delete assigned role request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		if IsNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to delete assigned role: %w", err)
	}

	return nil
}
