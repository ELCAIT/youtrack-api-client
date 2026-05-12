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
	timeTrackingSettingsFields = "id,workTimeSettings(id,minutesADay,workDays,firstDayOfWeek,daysAWeek),workItemTypes(id,name,autoAttached),attributePrototypes(id,name,values(id,name,description,autoAttach),instances(id,name,ordinal,values(id,name,description,autoAttach)))"
	workTimeSettingsFields     = "id,minutesADay,workDays,firstDayOfWeek,daysAWeek"
	workItemTypeFields         = "id,name,autoAttached"

	timeTrackingSettingsPath = "%s/api/admin/timeTrackingSettings?fields=" + timeTrackingSettingsFields
	workTimeSettingsPath     = "%s/api/admin/timeTrackingSettings/workTimeSettings?fields=" + workTimeSettingsFields
	workItemTypesPath        = "%s/api/admin/timeTrackingSettings/workItemTypes?fields=" + workItemTypeFields
	workItemTypeByIDPath     = "%s/api/admin/timeTrackingSettings/workItemTypes/%s?fields=" + workItemTypeFields
	workItemTypeByIDBasePath = "%s/api/admin/timeTrackingSettings/workItemTypes/%s"
)

// GetGlobalTimeTrackingSettings returns all global time tracking settings.
func (c *Client) GetGlobalTimeTrackingSettings(ctx context.Context) (GlobalTimeTrackingSettings, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(timeTrackingSettingsPath, c.HostURL), nil)
	if err != nil {
		return GlobalTimeTrackingSettings{}, fmt.Errorf("failed to create get global time tracking settings request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return GlobalTimeTrackingSettings{}, fmt.Errorf("failed to get global time tracking settings: %w", err)
	}

	var response GlobalTimeTrackingSettings
	if err := json.Unmarshal(body, &response); err != nil {
		return GlobalTimeTrackingSettings{}, fmt.Errorf("failed to unmarshal global time tracking settings response: %w", err)
	}

	return response, nil
}

// GetWorkTimeSettings returns global work schedule settings.
func (c *Client) GetWorkTimeSettings(ctx context.Context) (WorkTimeSettings, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(workTimeSettingsPath, c.HostURL), nil)
	if err != nil {
		return WorkTimeSettings{}, fmt.Errorf("failed to create get work time settings request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return WorkTimeSettings{}, fmt.Errorf("failed to get work time settings: %w", err)
	}

	var response WorkTimeSettings
	if err := json.Unmarshal(body, &response); err != nil {
		return WorkTimeSettings{}, fmt.Errorf("failed to unmarshal work time settings response: %w", err)
	}

	return response, nil
}

// UpdateWorkTimeSettings updates global work schedule settings.
func (c *Client) UpdateWorkTimeSettings(ctx context.Context, settings WorkTimeSettings) (WorkTimeSettings, error) {
	requestBody := map[string]interface{}{
		"minutesADay": settings.MinutesADay,
		"workDays":    settings.WorkDays,
	}

	rb, err := json.Marshal(requestBody)
	if err != nil {
		return WorkTimeSettings{}, fmt.Errorf("failed to marshal work time settings request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(workTimeSettingsPath, c.HostURL), bytes.NewReader(rb))
	if err != nil {
		return WorkTimeSettings{}, fmt.Errorf("failed to create update work time settings request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		return WorkTimeSettings{}, fmt.Errorf("failed to update work time settings: %w", err)
	}

	waitForAsyncProcessing()

	return c.GetWorkTimeSettings(ctx)
}

// ListWorkItemTypes returns all global work item types.
func (c *Client) ListWorkItemTypes(ctx context.Context) ([]WorkItemType, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(workItemTypesPath, c.HostURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list work item types request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list work item types: %w", err)
	}

	var response []WorkItemType
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal work item types response: %w", err)
	}

	return response, nil
}

// CreateWorkItemType creates a new global work item type.
func (c *Client) CreateWorkItemType(ctx context.Context, workItemType WorkItemType) (WorkItemType, error) {
	requestBody := map[string]interface{}{
		"name":         workItemType.Name,
		"autoAttached": workItemType.AutoAttached,
	}

	rb, err := json.Marshal(requestBody)
	if err != nil {
		return WorkItemType{}, fmt.Errorf("failed to marshal create work item type request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(workItemTypesPath, c.HostURL), bytes.NewReader(rb))
	if err != nil {
		return WorkItemType{}, fmt.Errorf("failed to create create work item type request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return WorkItemType{}, fmt.Errorf("failed to create work item type: %w", err)
	}

	var response WorkItemType
	if err := json.Unmarshal(body, &response); err != nil {
		return WorkItemType{}, fmt.Errorf("failed to unmarshal create work item type response: %w", err)
	}

	return response, nil
}

// UpdateWorkItemType updates a global work item type by ID.
func (c *Client) UpdateWorkItemType(ctx context.Context, workItemType WorkItemType) (WorkItemType, error) {
	if workItemType.ID == "" {
		return WorkItemType{}, fmt.Errorf("work item type ID is required for update")
	}

	requestBody := map[string]interface{}{
		"name":         workItemType.Name,
		"autoAttached": workItemType.AutoAttached,
	}

	rb, err := json.Marshal(requestBody)
	if err != nil {
		return WorkItemType{}, fmt.Errorf("failed to marshal update work item type request: %w", err)
	}

	path := fmt.Sprintf(workItemTypeByIDPath, c.HostURL, url.PathEscape(workItemType.ID))
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, path, bytes.NewReader(rb))
	if err != nil {
		return WorkItemType{}, fmt.Errorf("failed to create update work item type request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return WorkItemType{}, fmt.Errorf("failed to update work item type: %w", err)
	}

	var response WorkItemType
	if err := json.Unmarshal(body, &response); err != nil {
		return WorkItemType{}, fmt.Errorf("failed to unmarshal update work item type response: %w", err)
	}

	return response, nil
}

// DeleteWorkItemType deletes a global work item type by ID.
func (c *Client) DeleteWorkItemType(ctx context.Context, typeID string) error {
	if typeID == "" {
		return fmt.Errorf("work item type ID is required for delete")
	}

	path := fmt.Sprintf(workItemTypeByIDBasePath, c.HostURL, url.PathEscape(typeID))
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete work item type request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		if IsNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to delete work item type: %w", err)
	}

	return nil
}
