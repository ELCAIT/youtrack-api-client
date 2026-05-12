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
	stateBundlesAPIPath      = "api/admin/customFieldSettings/bundles/state"
	stateBundleByIDPath      = "%s/%s/%s?%s"
	stateBundleFieldsPath    = pathWithFieldsFormat
	stateBundlePagePath      = "%s/%s?%s&$top=%d&$skip=%d"
	stateBundleFieldsParam   = "fields=id,name,isUpdateable,values(id,name,localizedName,description,isResolved,archived,ordinal),$type"
	stateBundlePageSize      = 100
	errMarshalStateBundle    = "failed to marshal state bundle: %w"
	errMarshalStateBundleVal = "failed to marshal state bundle value: %w"
)

var errStateBundleNotFound = errors.New("state bundle not found")

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
func (c *Client) DeleteStateBundle(ctx context.Context, id string) error {
	return deleteByID(ctx, c, id, deleteConfig{
		HostURL:   c.HostURL,
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

	endpoint := fmt.Sprintf("%s/%s/%s/values/%s?%s", c.HostURL, stateBundlesAPIPath, bundleID, elementID, stateBundleFieldsParam)
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
