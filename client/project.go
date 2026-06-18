// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	projectsAPIPath    = "api/admin/projects"
	projectFieldsParam = "fields=id,name,shortName,description,leader(id,login,name),archived,template,fromEmail,replyToEmail,$type"

	projectCustomFieldsSubPath = "customFields"
	projectCustomFieldFields   = "fields=id,field(id,name),bundle(id,name,$type),defaultValues(id,name,$type),canBeEmpty,emptyFieldText,isPublic,$type"
	projectCustomFieldByIDFmt  = "%s/%s/%s/%s/%s?%s"
	projectCustomFieldsListFmt = "%s/%s/%s/%s?%s"

	projectTimeTrackingSubPath = "timeTrackingSettings"
	projectTimeTrackingFields  = "fields=id,enabled,estimate(id,field(id,name)),timeSpent(id,field(id,name)),$type"
	projectTimeTrackingFmt     = "%s/%s/%s/%s?%s"

	errMarshalProject             = "failed to marshal project: %w"
	errMarshalProjectCustomField  = "failed to marshal project custom field: %w"
	errMarshalProjectTimeTracking = "failed to marshal project time tracking settings: %w"
)

// UserRef represents a minimal reference to a YouTrack user.
type UserRef struct {
	ID    string `json:"id,omitempty"`
	Login string `json:"login,omitempty"`
	Name  string `json:"name,omitempty"`
	Type  string `json:"$type,omitempty"`
}

// Project represents a YouTrack project.
type Project struct {
	ID           string   `json:"id,omitempty"`
	Name         string   `json:"name,omitempty"`
	ShortName    string   `json:"shortName,omitempty"`
	Description  string   `json:"description,omitempty"`
	Leader       *UserRef `json:"leader,omitempty"`
	Archived     bool     `json:"archived,omitempty"`
	Template     bool     `json:"template,omitempty"`
	FromEmail    string   `json:"fromEmail,omitempty"`
	ReplyToEmail string   `json:"replyToEmail,omitempty"`
	Type         string   `json:"$type,omitempty"`
}

// ProjectCreatePayload is the request body for creating a project.
type ProjectCreatePayload struct {
	Name        string   `json:"name"`
	ShortName   string   `json:"shortName"`
	Description string   `json:"description,omitempty"`
	Leader      *UserRef `json:"leader"`
	Template    *bool    `json:"template,omitempty"`
}

// ProjectUpdatePayload is the request body for updating a project.
type ProjectUpdatePayload struct {
	Name         string   `json:"name,omitempty"`
	Description  string   `json:"description,omitempty"`
	Leader       *UserRef `json:"leader,omitempty"`
	Archived     *bool    `json:"archived,omitempty"`
	FromEmail    string   `json:"fromEmail,omitempty"`
	ReplyToEmail string   `json:"replyToEmail,omitempty"`
}

// CustomFieldIDRef is a minimal reference to a global CustomField (used in ProjectCustomField.Field).
type CustomFieldIDRef struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"$type,omitempty"`
}

// ProjectCustomFieldValueRef is a minimal reference to a project custom field default value.
type ProjectCustomFieldValueRef struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"$type,omitempty"`
}

// ProjectCustomField represents a custom field attached to a project.
type ProjectCustomField struct {
	ID             string                       `json:"id,omitempty"`
	Field          *CustomFieldIDRef            `json:"field,omitempty"`
	Bundle         *BundleRef                   `json:"bundle,omitempty"`
	DefaultValues  []ProjectCustomFieldValueRef `json:"defaultValues,omitempty"`
	CanBeEmpty     bool                         `json:"canBeEmpty,omitempty"`
	EmptyFieldText string                       `json:"emptyFieldText,omitempty"`
	IsPublic       bool                         `json:"isPublic,omitempty"`
	Type           string                       `json:"$type,omitempty"`
}

// ProjectCustomFieldUpsertPayload is the request body for attaching or updating a project custom field.
type ProjectCustomFieldUpsertPayload struct {
	Field          *CustomFieldIDRef            `json:"field,omitempty"`
	Bundle         *BundleRef                   `json:"bundle,omitempty"`
	DefaultValues  []ProjectCustomFieldValueRef `json:"defaultValues,omitempty"`
	CanBeEmpty     *bool                        `json:"canBeEmpty,omitempty"`
	EmptyFieldText string                       `json:"emptyFieldText,omitempty"`
	IsPublic       *bool                        `json:"isPublic,omitempty"`
	Type           string                       `json:"$type,omitempty"`
}

// ProjectCustomFieldTimeRef is a reference to a ProjectCustomField used in time tracking settings.
type ProjectCustomFieldTimeRef struct {
	ID    string            `json:"id,omitempty"`
	Field *CustomFieldIDRef `json:"field,omitempty"`
	Type  string            `json:"$type,omitempty"`
}

// ProjectTimeTrackingSettings represents the time tracking settings for a project.
type ProjectTimeTrackingSettings struct {
	ID        string                     `json:"id,omitempty"`
	Enabled   bool                       `json:"enabled,omitempty"`
	Estimate  *ProjectCustomFieldTimeRef `json:"estimate,omitempty"`
	TimeSpent *ProjectCustomFieldTimeRef `json:"timeSpent,omitempty"`
	Type      string                     `json:"$type,omitempty"`
}

// ProjectTimeTrackingUpdatePayload is the request body for updating project time tracking settings.
type ProjectTimeTrackingUpdatePayload struct {
	Enabled   bool                       `json:"enabled"`
	Estimate  *ProjectCustomFieldTimeRef `json:"estimate,omitempty"`
	TimeSpent *ProjectCustomFieldTimeRef `json:"timeSpent,omitempty"`
}

// CreateProject creates a new project in YouTrack.
func (c *Client) CreateProject(ctx context.Context, payload ProjectCreatePayload) (*Project, error) {
	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(errMarshalProject, err)
	}

	url := fmt.Sprintf(pathWithFieldsFormat, c.HostURL, projectsAPIPath, projectFieldsParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, url, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create project request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	var created Project
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("failed to unmarshal create project response: %w", err)
	}

	return &created, nil
}

// GetProject reads a specific project by its entity ID.
func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	url := fmt.Sprintf(specificIssueLinkType, c.HostURL, projectsAPIPath, id, projectFieldsParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get project request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	var project Project
	if err := json.Unmarshal(body, &project); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project response: %w", err)
	}

	return &project, nil
}

// UpdateProject updates a specific project.
func (c *Client) UpdateProject(ctx context.Context, id string, payload ProjectUpdatePayload) (*Project, error) {
	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(errMarshalProject, err)
	}

	url := fmt.Sprintf(specificIssueLinkType, c.HostURL, projectsAPIPath, id, projectFieldsParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, url, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update project request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	var updated Project
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, fmt.Errorf("failed to unmarshal update project response: %w", err)
	}

	return &updated, nil
}

// DeleteProject deletes a specific project.
func (c *Client) DeleteProject(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/%s/%s", c.HostURL, projectsAPIPath, id)
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete project request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	return nil
}

// GetProjectCustomFields returns all custom fields attached to a project.
func (c *Client) GetProjectCustomFields(ctx context.Context, projectID string) ([]ProjectCustomField, error) {
	url := fmt.Sprintf(projectCustomFieldsListFmt, c.HostURL, projectsAPIPath, projectID, projectCustomFieldsSubPath, projectCustomFieldFields)
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get project custom fields request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get project custom fields: %w", err)
	}

	var fields []ProjectCustomField
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project custom fields response: %w", err)
	}

	return fields, nil
}

// GetProjectCustomFieldByName returns the project custom field attachment with the given global field name.
func (c *Client) GetProjectCustomFieldByName(ctx context.Context, projectID, fieldName string) (*ProjectCustomField, error) {
	fields, err := c.GetProjectCustomFields(ctx, projectID)
	if err != nil {
		return nil, err
	}

	for i := range fields {
		if fields[i].Field != nil && fields[i].Field.Name == fieldName {
			return &fields[i], nil
		}
	}

	return nil, fmt.Errorf("project custom field with name %q not found in project %q", fieldName, projectID)
}

// GetProjectCustomField reads a specific custom field attachment in a project.
func (c *Client) GetProjectCustomField(ctx context.Context, projectID, fieldID string) (*ProjectCustomField, error) {
	url := fmt.Sprintf(projectCustomFieldByIDFmt, c.HostURL, projectsAPIPath, projectID, projectCustomFieldsSubPath, fieldID, projectCustomFieldFields)
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get project custom field request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get project custom field: %w", err)
	}

	var field ProjectCustomField
	if err := json.Unmarshal(body, &field); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project custom field response: %w", err)
	}

	return &field, nil
}

// AddProjectCustomField attaches a custom field to a project.
func (c *Client) AddProjectCustomField(ctx context.Context, projectID string, payload ProjectCustomFieldUpsertPayload) (*ProjectCustomField, error) {
	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(errMarshalProjectCustomField, err)
	}

	url := fmt.Sprintf(projectCustomFieldsListFmt, c.HostURL, projectsAPIPath, projectID, projectCustomFieldsSubPath, projectCustomFieldFields)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, url, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create add project custom field request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to add project custom field: %w", err)
	}

	var created ProjectCustomField
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("failed to unmarshal add project custom field response: %w", err)
	}

	return &created, nil
}

// UpdateProjectCustomField updates a custom field attachment in a project.
func (c *Client) UpdateProjectCustomField(ctx context.Context, projectID, fieldID string, payload ProjectCustomFieldUpsertPayload) (*ProjectCustomField, error) {
	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(errMarshalProjectCustomField, err)
	}

	url := fmt.Sprintf(projectCustomFieldByIDFmt, c.HostURL, projectsAPIPath, projectID, projectCustomFieldsSubPath, fieldID, projectCustomFieldFields)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, url, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update project custom field request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update project custom field: %w", err)
	}

	var updated ProjectCustomField
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, fmt.Errorf("failed to unmarshal update project custom field response: %w", err)
	}

	return &updated, nil
}

// RemoveProjectCustomField removes a custom field attachment from a project.
func (c *Client) RemoveProjectCustomField(ctx context.Context, projectID, fieldID string) error {
	url := fmt.Sprintf("%s/%s/%s/%s/%s", c.HostURL, projectsAPIPath, projectID, projectCustomFieldsSubPath, fieldID)
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create remove project custom field request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to remove project custom field: %w", err)
	}

	return nil
}

// GetProjectTimeTrackingSettings reads the time tracking settings for a project.
func (c *Client) GetProjectTimeTrackingSettings(ctx context.Context, projectID string) (*ProjectTimeTrackingSettings, error) {
	url := fmt.Sprintf(projectTimeTrackingFmt, c.HostURL, projectsAPIPath, projectID, projectTimeTrackingSubPath, projectTimeTrackingFields)
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get project time tracking settings request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get project time tracking settings: %w", err)
	}

	var settings ProjectTimeTrackingSettings
	if err := json.Unmarshal(body, &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project time tracking settings response: %w", err)
	}

	return &settings, nil
}

// UpdateProjectTimeTrackingSettings updates the time tracking settings for a project.
func (c *Client) UpdateProjectTimeTrackingSettings(ctx context.Context, projectID string, payload ProjectTimeTrackingUpdatePayload) (*ProjectTimeTrackingSettings, error) {
	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(errMarshalProjectTimeTracking, err)
	}

	url := fmt.Sprintf(projectTimeTrackingFmt, c.HostURL, projectsAPIPath, projectID, projectTimeTrackingSubPath, projectTimeTrackingFields)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, url, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update project time tracking settings request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update project time tracking settings: %w", err)
	}

	var updated ProjectTimeTrackingSettings
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, fmt.Errorf("failed to unmarshal update project time tracking settings response: %w", err)
	}

	return &updated, nil
}
