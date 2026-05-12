package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	roleApiPath          = "api/assignedRoles"
	youtrackRolesAPIPath = "api/roles"
	fieldsQueryParam     = "fields=id,role(id,name,description),scope(id,$type),holder(id,name,login,$type),$type"
	pathWithFieldsFormat = "%s/%s?%s"
	allAssignedRoles     = pathWithFieldsFormat
	specificAssignedRole = "%s/%s/%s?%s"
)

// GetAllAssignedRoles - Returns list of assigned roles.
func (c *Client) GetAllAssignedRoles(ctx context.Context) ([]AssignedRoles, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(allAssignedRoles, c.HostURL, roleApiPath, fieldsQueryParam), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get roles request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles: %w", err)
	}

	var response AssignedRolesResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal roles response: %w", err)
	}

	// Return the roles from the response
	return response.AssignedRoles, nil
}

// GetAssignedRoleById - Returns a specific assigned role by ID.
func (c *Client) GetAssignedRoleById(ctx context.Context, roleAssignmentId string) (*AssignedRoles, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(specificAssignedRole, c.HostURL, roleApiPath, roleAssignmentId, fieldsQueryParam), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get assigned role request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get assigned role: %w", err)
	}

	var assignedRole AssignedRoles
	err = json.Unmarshal(body, &assignedRole)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal assigned role response: %w", err)
	}

	return &assignedRole, nil
}

// CreateAssignedRole - Creates a new role assignment.
func (c *Client) CreateAssignedRole(ctx context.Context, assignedRole AssignedRoles) (*AssignedRoles, error) {
	rb, err := json.Marshal(assignedRole)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal assigned role: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(allAssignedRoles, c.HostURL, roleApiPath, fieldsQueryParam), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create assigned role request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create assigned role with payload %s: %w", string(rb), err)
	}

	var createdRole AssignedRoles
	err = json.Unmarshal(body, &createdRole)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal created assigned role: %w", err)
	}

	return &createdRole, nil
}

// UpdateAssignedRole - Updates an existing role assignment.
func (c *Client) UpdateAssignedRole(ctx context.Context, roleAssignmentId string, assignedRole AssignedRoles) (*AssignedRoles, error) {
	rb, err := json.Marshal(assignedRole)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal assigned role: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(specificAssignedRole, c.HostURL, roleApiPath, roleAssignmentId, fieldsQueryParam), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update assigned role request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update assigned role with payload %s: %w", string(rb), err)
	}

	var updatedRole AssignedRoles
	err = json.Unmarshal(body, &updatedRole)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated assigned role: %w", err)
	}

	return &updatedRole, nil
}

// DeleteAssignedRole - Deletes a role assignment.
func (c *Client) DeleteAssignedRole(ctx context.Context, roleAssignmentId string) error {
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, fmt.Sprintf(specificAssignedRole, c.HostURL, roleApiPath, roleAssignmentId, fieldsQueryParam), nil)
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
