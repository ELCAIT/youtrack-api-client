package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	issueLinkTypesAPIPath    = "api/issueLinkTypes"
	issueLinkTypeFieldsParam = "fields=id,name,sourceToTarget,targetToSource,directed,aggregation,readOnly,localizedName,localizedSourceToTarget,localizedTargetToSource,$type"
	allIssueLinkTypesPath    = pathWithFieldsFormat
	specificIssueLinkType    = "%s/%s/%s?%s"

	errMarshalIssueLinkType = "failed to marshal issue link type: %w"
)

// IssueLinkType represents a YouTrack issue link type.
type IssueLinkType struct {
	ID                      string `json:"id,omitempty"`
	Name                    string `json:"name,omitempty"`
	SourceToTarget          string `json:"sourceToTarget,omitempty"`
	TargetToSource          string `json:"targetToSource,omitempty"`
	Directed                bool   `json:"directed,omitempty"`
	Aggregation             bool   `json:"aggregation,omitempty"`
	ReadOnly                bool   `json:"readOnly,omitempty"`
	LocalizedName           string `json:"localizedName,omitempty"`
	LocalizedSourceToTarget string `json:"localizedSourceToTarget,omitempty"`
	LocalizedTargetToSource string `json:"localizedTargetToSource,omitempty"`
	Type                    string `json:"$type,omitempty"`
}

// GetAllIssueLinkTypes returns all issue link types.
func (c *Client) GetAllIssueLinkTypes(ctx context.Context) ([]IssueLinkType, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet,
		fmt.Sprintf(allIssueLinkTypesPath, c.HostURL, issueLinkTypesAPIPath, issueLinkTypeFieldsParam), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get issue link types request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue link types: %w", err)
	}

	var issueLinkTypes []IssueLinkType
	if err := json.Unmarshal(body, &issueLinkTypes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal issue link types response: %w", err)
	}

	return issueLinkTypes, nil
}

// GetIssueLinkTypeByID returns a specific issue link type by ID.
func (c *Client) GetIssueLinkTypeByID(ctx context.Context, id string) (*IssueLinkType, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet,
		fmt.Sprintf(specificIssueLinkType, c.HostURL, issueLinkTypesAPIPath, id, issueLinkTypeFieldsParam), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get issue link type request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue link type: %w", err)
	}

	var issueLinkType IssueLinkType
	if err := json.Unmarshal(body, &issueLinkType); err != nil {
		return nil, fmt.Errorf("failed to unmarshal issue link type response: %w", err)
	}

	return &issueLinkType, nil
}

// CreateIssueLinkType creates a new issue link type.
func (c *Client) CreateIssueLinkType(ctx context.Context, issueLinkType IssueLinkType) (*IssueLinkType, error) {
	payload := IssueLinkType{
		Name:                    issueLinkType.Name,
		SourceToTarget:          issueLinkType.SourceToTarget,
		TargetToSource:          issueLinkType.TargetToSource,
		Directed:                issueLinkType.Directed,
		Aggregation:             issueLinkType.Aggregation,
		LocalizedName:           issueLinkType.LocalizedName,
		LocalizedSourceToTarget: issueLinkType.LocalizedSourceToTarget,
		LocalizedTargetToSource: issueLinkType.LocalizedTargetToSource,
	}

	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(errMarshalIssueLinkType, err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost,
		fmt.Sprintf(allIssueLinkTypesPath, c.HostURL, issueLinkTypesAPIPath, issueLinkTypeFieldsParam), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create create issue link type request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue link type: %w", err)
	}

	var created IssueLinkType
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created issue link type: %w", err)
	}

	return &created, nil
}

// UpdateIssueLinkType updates an existing issue link type by ID.
func (c *Client) UpdateIssueLinkType(ctx context.Context, id string, issueLinkType IssueLinkType) (*IssueLinkType, error) {
	payload := IssueLinkType{
		Name:                    issueLinkType.Name,
		SourceToTarget:          issueLinkType.SourceToTarget,
		TargetToSource:          issueLinkType.TargetToSource,
		Directed:                issueLinkType.Directed,
		Aggregation:             issueLinkType.Aggregation,
		LocalizedName:           issueLinkType.LocalizedName,
		LocalizedSourceToTarget: issueLinkType.LocalizedSourceToTarget,
		LocalizedTargetToSource: issueLinkType.LocalizedTargetToSource,
	}

	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(errMarshalIssueLinkType, err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost,
		fmt.Sprintf(specificIssueLinkType, c.HostURL, issueLinkTypesAPIPath, id, issueLinkTypeFieldsParam), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update issue link type request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update issue link type: %w", err)
	}

	var updated IssueLinkType
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated issue link type: %w", err)
	}

	return &updated, nil
}

// DeleteIssueLinkType deletes a specific issue link type by ID.
func (c *Client) DeleteIssueLinkType(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete,
		fmt.Sprintf("%s/%s/%s", c.HostURL, issueLinkTypesAPIPath, id), nil)
	if err != nil {
		return fmt.Errorf("failed to create delete issue link type request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		if IsNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to delete issue link type: %w", err)
	}

	return nil
}
