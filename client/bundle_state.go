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
	stateBundlesAPIPath      = "api/admin/customFieldSettings/bundles/state"
	stateBundleByIDPath      = "%s/%s/%s?%s"
	stateBundleFieldsPath    = pathWithFieldsFormat
	stateBundlePagePath      = "%s/%s?%s&$top=%d&$skip=%d"
	stateBundleValueFields   = "id,name,localizedName,description,isResolved,archived,ordinal,$type"
	stateBundleFieldsParam   = "fields=id,name,isUpdateable,values(" + stateBundleValueFields + "),$type"
	stateBundlePageSize      = 100
	errMarshalStateBundle    = "failed to marshal state bundle: %w"
	errMarshalStateBundleVal = "failed to marshal state bundle value: %w"
)

var errStateBundleNotFound = fmt.Errorf("state bundle %w", ErrNotFound)

// StateBundleElement represents a single state value inside a state bundle.
type StateBundleElement struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	LocalizedName string `json:"localizedName,omitempty"`
	Description   string `json:"description,omitempty"`
	IsResolved    bool   `json:"isResolved,omitempty"`
	Archived      bool   `json:"archived,omitempty"`
	Ordinal       int    `json:"ordinal,omitempty"`
	Type          string `json:"$type,omitempty"`
}

// StateBundleValueUpdate is the request body for ReplaceStateBundleValue. It
// replaces every field it carries: a nil LocalizedName or Description is sent
// as null and clears it, and Archived is always sent. IsResolved is always sent too. Ordinal is only
// sent when set.
type StateBundleValueUpdate struct {
	Name          string  `json:"name"`
	LocalizedName *string `json:"localizedName"`
	Description   *string `json:"description"`
	IsResolved    bool    `json:"isResolved"`
	Archived      bool    `json:"archived"`
	Ordinal       *int    `json:"ordinal,omitempty"`
}

// StateBundle represents a YouTrack state bundle.
type StateBundle struct {
	ID           string               `json:"id,omitempty"`
	Name         string               `json:"name,omitempty"`
	IsUpdateable bool                 `json:"isUpdateable,omitempty"`
	Values       []StateBundleElement `json:"values,omitempty"`
	Type         string               `json:"$type,omitempty"`
}

// GetStateBundleByID returns a specific state bundle.
func (c *Client) GetStateBundleByID(ctx context.Context, id string) (*StateBundle, error) {
	return fetchAndDecodeByID[StateBundle](
		ctx,
		c,
		idFetchConfig{
			PathFormat: stateBundleByIDPath,
			HostURL:    c.HostURL,
			APIPath:    stateBundlesAPIPath,
			Fields:     stateBundleFieldsParam,
			ErrCreate:  "failed to create get state bundle request: %w",
			ErrFetch:   "failed to get state bundle: %w",
			ErrDecode:  "failed to unmarshal state bundle response: %w",
		},
		id,
	)
}

// GetStateBundleByName returns a state bundle by name.
func (c *Client) GetStateBundleByName(ctx context.Context, name string) (*StateBundle, error) {
	bundle, err := lookupByNamePaginated(ctx, stateBundlePageSize, name, c.getStateBundlePage, stateBundleName)
	if err != nil {
		return nil, err
	}
	if bundle != nil {
		return bundle, nil
	}

	return nil, fmt.Errorf("%w: name '%s'", errStateBundleNotFound, name)
}

func (c *Client) getStateBundlePage(ctx context.Context, skip int) ([]StateBundle, error) {
	return fetchAndDecodePage(
		ctx,
		c,
		pageFetchConfig{
			PathFormat: stateBundlePagePath,
			HostURL:    c.HostURL,
			APIPath:    stateBundlesAPIPath,
			Fields:     stateBundleFieldsParam,
			PageSize:   stateBundlePageSize,
			ErrCreate:  "failed to create get state bundles request: %w",
			ErrFetch:   "failed to get state bundles: %w",
		},
		skip,
		decodeStateBundles,
	)
}

func stateBundleName(bundle StateBundle) string {
	return bundle.Name
}

func decodeStateBundles(body []byte) ([]StateBundle, error) {
	return decodeBundleList[StateBundle](body, "failed to unmarshal state bundles response: %w")
}

// IsStateBundleNotFoundError checks whether an error indicates that a state bundle could not be found by name.
func IsStateBundleNotFoundError(err error) bool {
	return errors.Is(err, errStateBundleNotFound)
}

// CreateStateBundle creates a state bundle.
func (c *Client) CreateStateBundle(ctx context.Context, bundle StateBundle) (*StateBundle, error) {
	return createAndDecode(ctx, c, bundle, createConfig{
		PathFormat: stateBundleFieldsPath,
		HostURL:    c.HostURL,
		APIPath:    stateBundlesAPIPath,
		Fields:     stateBundleFieldsParam,
		ErrMarshal: errMarshalStateBundle,
		ErrCreate:  "failed to create create state bundle request: %w",
		ErrFetch:   "failed to create state bundle: %w",
		ErrDecode:  "failed to unmarshal created state bundle: %w",
	})
}

// UpdateStateBundle updates a state bundle by ID.
func (c *Client) UpdateStateBundle(ctx context.Context, id string, bundle StateBundle) (*StateBundle, error) {
	return updateAndDecode(ctx, c, id, bundle, updateConfig{
		PathFormat: stateBundleByIDPath,
		HostURL:    c.HostURL,
		APIPath:    stateBundlesAPIPath,
		Fields:     stateBundleFieldsParam,
		ErrMarshal: errMarshalStateBundle,
		ErrCreate:  "failed to create update state bundle request: %w",
		ErrFetch:   "failed to update state bundle: %w",
		ErrDecode:  "failed to unmarshal updated state bundle: %w",
	})
}

// DeleteStateBundle deletes a state bundle by ID.
// While YouTrack still reports the bundle as in use, shortly after the last
// field using it was removed, the delete is retried.
func (c *Client) DeleteStateBundle(ctx context.Context, id string) error {
	return deleteBundleByID(ctx, c, id, deleteConfig{
		APIPath:   stateBundlesAPIPath,
		ErrCreate: "failed to create delete state bundle request: %w",
		ErrFetch:  "failed to delete state bundle: %w",
	})
}

// UpdateStateBundleValue updates a specific value in a state bundle.
func (c *Client) UpdateStateBundleValue(ctx context.Context, bundleID, elementID string, value StateBundleElement) (*StateBundleElement, error) {
	rb, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf(errMarshalStateBundleVal, err)
	}

	endpoint := fmt.Sprintf("%s/%s/%s/values/%s?%s", c.HostURL, stateBundlesAPIPath, url.PathEscape(bundleID), url.PathEscape(elementID), stateBundleFieldsParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update state bundle value request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update state bundle value: %w", err)
	}

	var updated StateBundleElement
	if err = json.Unmarshal(body, &updated); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated state bundle value: %w", err)
	}

	return &updated, nil
}

var stateBundleValues = bundleValueEndpoint{APIPath: stateBundlesAPIPath, Fields: stateBundleValueFields, Kind: "state"}

// AddStateBundleValue adds a value to a state bundle.
func (c *Client) AddStateBundleValue(ctx context.Context, bundleID string, value StateBundleElement) (*StateBundleElement, error) {
	return addBundleValue[StateBundleElement](ctx, c, stateBundleValues, bundleID, value)
}

// ReplaceStateBundleValue replaces the fields of one value in a state bundle.
func (c *Client) ReplaceStateBundleValue(ctx context.Context, bundleID, valueID string, value StateBundleValueUpdate) (*StateBundleElement, error) {
	return replaceBundleValue[StateBundleElement](ctx, c, stateBundleValues, bundleID, valueID, value)
}

// DeleteStateBundleValue removes one value from a state bundle, waiting until
// YouTrack no longer lists it. A value that is already gone is treated as
// deleted.
func (c *Client) DeleteStateBundleValue(ctx context.Context, bundleID, valueID string) error {
	return deleteBundleValue(ctx, c, stateBundleValues, bundleID, valueID, func(ctx context.Context) (bool, error) {
		bundle, err := c.GetStateBundleByID(ctx, bundleID)
		if err != nil {
			return false, err
		}
		return bundleListsValue(bundle.Values, valueID, func(v StateBundleElement) string { return v.ID }), nil
	})
}
