package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

const (
	customFieldsAPIPath    = "api/admin/customFieldSettings/customFields"
	customFieldByIDPath    = "%s/%s/%s?%s"
	customFieldFieldsPath  = pathWithFieldsFormat
	customFieldPagePath    = "%s/%s?%s&$top=%d&$skip=%d"
	customFieldFieldsParam = "fields=id,name,localizedName,aliases,fieldType(id,presentation)," +
		"isAutoAttached,isDisplayedInIssueList,ordinal,isUpdateable,hasRunningJob," +
		"fieldDefaults(id,canBeEmpty,emptyFieldText,isPublic,bundle(id,name),defaultValues(id,name,$type)),$type"
	customFieldNameFields = "fields=id,name"
	customFieldPageSize   = 100

	errMarshalCustomField = "failed to marshal custom field: %w"
)

var errCustomFieldNotFound = errors.New("custom field not found")

// FieldType represents a YouTrack custom field type descriptor.
type FieldType struct {
	ID           string `json:"id,omitempty"`
	Presentation string `json:"presentation,omitempty"`
	Type         string `json:"$type,omitempty"`
}

// BundleRef represents a reference to a YouTrack bundle.
type BundleRef struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"$type,omitempty"`
}

// CustomFieldDefaults represents the default settings for a custom field.
type CustomFieldDefaults struct {
	ID             string                       `json:"id,omitempty"`
	CanBeEmpty     bool                         `json:"canBeEmpty,omitempty"`
	EmptyFieldText string                       `json:"emptyFieldText,omitempty"`
	IsPublic       bool                         `json:"isPublic,omitempty"`
	Bundle         *BundleRef                   `json:"bundle,omitempty"`
	DefaultValues  []ProjectCustomFieldValueRef `json:"defaultValues,omitempty"`
	Type           string                       `json:"$type,omitempty"`
}

// CustomField represents a custom field in YouTrack.
type CustomField struct {
	ID                     string               `json:"id,omitempty"`
	Name                   string               `json:"name,omitempty"`
	LocalizedName          string               `json:"localizedName,omitempty"`
	Aliases                string               `json:"aliases,omitempty"`
	FieldType              FieldType            `json:"fieldType,omitempty"`
	IsAutoAttached         bool                 `json:"isAutoAttached,omitempty"`
	IsDisplayedInIssueList bool                 `json:"isDisplayedInIssueList,omitempty"`
	Ordinal                int                  `json:"ordinal,omitempty"`
	IsUpdateable           bool                 `json:"isUpdateable,omitempty"`
	HasRunningJob          bool                 `json:"hasRunningJob,omitempty"`
	FieldDefaults          *CustomFieldDefaults `json:"fieldDefaults,omitempty"`
	Type                   string               `json:"$type,omitempty"`
}

// CustomFieldUpsertRequest contains attributes supported by create/update operations.
type CustomFieldUpsertRequest struct {
	Name                   string                          `json:"name,omitempty"`
	LocalizedName          *string                         `json:"localizedName,omitempty"`
	Aliases                *string                         `json:"aliases,omitempty"`
	FieldType              *FieldType                      `json:"fieldType,omitempty"`
	IsAutoAttached         *bool                           `json:"isAutoAttached,omitempty"`
	IsDisplayedInIssueList *bool                           `json:"isDisplayedInIssueList,omitempty"`
	FieldDefaults          *CustomFieldDefaultsUpsertModel `json:"fieldDefaults,omitempty"`
}

// CustomFieldDefaultsUpsertModel contains writable default settings.
type CustomFieldDefaultsUpsertModel struct {
	CanBeEmpty     *bool                        `json:"canBeEmpty,omitempty"`
	EmptyFieldText *string                      `json:"emptyFieldText,omitempty"`
	IsPublic       *bool                        `json:"isPublic,omitempty"`
	Bundle         *BundleRef                   `json:"bundle,omitempty"`
	DefaultValues  []ProjectCustomFieldValueRef `json:"defaultValues,omitempty"`
	Type           string                       `json:"$type,omitempty"`
}

// GetCustomFieldByID returns a specific custom field by ID.
func (c *Client) GetCustomFieldByID(ctx context.Context, id string) (*CustomField, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet,
		fmt.Sprintf(customFieldByIDPath, c.HostURL, customFieldsAPIPath, id, customFieldFieldsParam), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get custom field request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get custom field: %w", err)
	}

	var customField CustomField
	if err = json.Unmarshal(body, &customField); err != nil {
		return nil, fmt.Errorf("failed to unmarshal custom field response: %w", err)
	}

	return &customField, nil
}

// IsCustomFieldNotFoundError checks whether an error indicates that a custom field could not be found by name.
func IsCustomFieldNotFoundError(err error) bool {
	return errors.Is(err, errCustomFieldNotFound)
}

// GetCustomFieldByName looks up a custom field by its exact name, paging through all custom fields.
func (c *Client) GetCustomFieldByName(ctx context.Context, name string) (*CustomField, error) {
	skip := 0
	for {
		url := fmt.Sprintf(customFieldPagePath, c.HostURL, customFieldsAPIPath, customFieldNameFields, customFieldPageSize, skip)

		req, err := http.NewRequestWithContext(ctx, httpMethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create list custom fields request: %w", err)
		}

		body, err := c.doRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to list custom fields: %w", err)
		}

		var page []CustomField
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("failed to unmarshal custom fields list: %w", err)
		}

		for _, f := range page {
			if f.Name == name {
				return &f, nil
			}
		}

		if len(page) < customFieldPageSize {
			break
		}

		skip += customFieldPageSize
	}

	return nil, fmt.Errorf("%w: %s", errCustomFieldNotFound, name)
}

// CreateCustomField creates a new custom field.
func (c *Client) CreateCustomField(ctx context.Context, customField CustomFieldUpsertRequest) (*CustomField, error) {
	rb, err := json.Marshal(customField)
	if err != nil {
		return nil, fmt.Errorf(errMarshalCustomField, err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost,
		fmt.Sprintf(customFieldFieldsPath, c.HostURL, customFieldsAPIPath, customFieldFieldsParam), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create create custom field request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create custom field: %w", err)
	}

	var created CustomField
	if err = json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created custom field: %w", err)
	}

	return &created, nil
}

// UpdateCustomField updates a custom field by ID.
func (c *Client) UpdateCustomField(ctx context.Context, id string, customField CustomFieldUpsertRequest) (*CustomField, error) {
	rb, err := json.Marshal(customField)
	if err != nil {
		return nil, fmt.Errorf(errMarshalCustomField, err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost,
		fmt.Sprintf(customFieldByIDPath, c.HostURL, customFieldsAPIPath, id, customFieldFieldsParam), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update custom field request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update custom field: %w", err)
	}

	var updated CustomField
	if err = json.Unmarshal(body, &updated); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated custom field: %w", err)
	}

	return &updated, nil
}

// DeleteCustomField deletes a custom field by ID.
func (c *Client) DeleteCustomField(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete,
		fmt.Sprintf("%s/%s/%s", c.HostURL, customFieldsAPIPath, id), nil)
	if err != nil {
		return fmt.Errorf("failed to create delete custom field request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		if IsNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to delete custom field: %w", err)
	}

	return nil
}
