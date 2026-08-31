package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	// hubGroupIdentityFields is the projection needed to reconcile a group against an
	// identity provider's own group id. The auth module is part of it because
	// idpGroupId is shared by every provider: it says which one a tagged group
	// belongs to.
	hubGroupIdentityFields = "id,name,description,idpGroupId,idpGroupName,importedFromAuthModule(id,name),mappedInAuthModule(id,name)"

	// hubGroupPageSize is the page size used when scanning groups for an identity.
	hubGroupPageSize = 500
)

// HubGroup is a user group as the Hub REST API represents it.
//
// Hub is where a group's identity-provider attributes live: the YouTrack group API
// neither returns idpGroupId nor accepts it on a write, so a group tagged with its id
// at an upstream directory has to be read and written through Hub.
type HubGroup struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	// IDPGroupID is the group's id at the identity provider, and the key for
	// reconciling that provider's role events against YouTrack groups.
	IDPGroupID string `json:"idpGroupId,omitempty"`
	// IDPGroupName is the group's name at the identity provider, kept alongside the
	// id so an operator can see what a tagged group corresponds to.
	IDPGroupName string `json:"idpGroupName,omitempty"`
	// ImportedFromAuthModule and MappedInAuthModule name the provider a tagged group
	// belongs to. Hub sets one or the other depending on how the group came to be
	// associated with the module, so a caller checks both.
	ImportedFromAuthModule *AuthModule `json:"importedFromAuthModule,omitempty"`
	MappedInAuthModule     *AuthModule `json:"mappedInAuthModule,omitempty"`
}

// AuthModuleName returns the name of the identity provider a group is tagged for, or
// the empty string when it is tagged for none.
func (g HubGroup) AuthModuleName() string {
	if g.ImportedFromAuthModule != nil && strings.TrimSpace(g.ImportedFromAuthModule.Name) != "" {
		return g.ImportedFromAuthModule.Name
	}
	if g.MappedInAuthModule != nil {
		return g.MappedInAuthModule.Name
	}

	return ""
}

// hubGroupsPage is one page of the Hub user groups listing.
type hubGroupsPage struct {
	UserGroups []HubGroup `json:"usergroups"`
	Total      int        `json:"total"`
}

// ListHubGroups returns one page of Hub user groups with their identity attributes.
func (c *Client) ListHubGroups(ctx context.Context, top, skip int) ([]HubGroup, error) {
	values := url.Values{}
	values.Set("fields", hubGroupIdentityFields)
	if top > 0 {
		values.Set("$top", strconv.Itoa(top))
	}
	if skip > 0 {
		values.Set("$skip", strconv.Itoa(skip))
	}

	endpoint := fmt.Sprintf("%s/%s?%s", c.HostURL, hubRestUserGroupsAPIPath, values.Encode())
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list hub groups request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list hub groups: %w", err)
	}

	var page hubGroupsPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("failed to unmarshal list hub groups response: %w", err)
	}

	return page.UserGroups, nil
}

// FindGroupByIDPGroupID returns the group carrying idpGroupID, or nil when none does.
//
// Hub cannot filter on the attribute server-side -- idpGroupId is not one of the fields
// its group query understands -- so the listing is scanned. Absence is not an error: a
// role that has no group yet is a normal state, and the caller tells it from a failure
// by the nil group.
//
// Every identity provider writes into the same idpGroupId field, so a group tagged by
// one provider can hold the id of an unrelated group at another. Pass authModuleName to
// restrict the match to one provider's groups; pass the empty string to match on the id
// alone, which is only safe when a single provider tags groups on the instance.
func (c *Client) FindGroupByIDPGroupID(ctx context.Context, authModuleName, idpGroupID string) (*HubGroup, error) {
	wanted := strings.TrimSpace(idpGroupID)
	if wanted == "" {
		return nil, fmt.Errorf("idp group id must not be empty")
	}
	module := strings.TrimSpace(authModuleName)

	for skip := 0; ; skip += hubGroupPageSize {
		groups, err := c.ListHubGroups(ctx, hubGroupPageSize, skip)
		if err != nil {
			return nil, err
		}

		for i := range groups {
			if !strings.EqualFold(strings.TrimSpace(groups[i].IDPGroupID), wanted) {
				continue
			}
			if module != "" && !strings.EqualFold(strings.TrimSpace(groups[i].AuthModuleName()), module) {
				continue
			}

			return &groups[i], nil
		}

		if len(groups) < hubGroupPageSize {
			return nil, nil
		}
	}
}

// GetHubGroup returns one Hub group with its identity attributes.
func (c *Client) GetHubGroup(ctx context.Context, groupID string) (*HubGroup, error) {
	if strings.TrimSpace(groupID) == "" {
		return nil, fmt.Errorf("group id must not be empty")
	}

	values := url.Values{}
	values.Set("fields", hubGroupIdentityFields)

	endpoint := fmt.Sprintf("%s/%s/%s?%s", c.HostURL, hubRestUserGroupsAPIPath, url.PathEscape(groupID), values.Encode())
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get hub group request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get hub group: %w", err)
	}

	var group HubGroup
	if err := json.Unmarshal(body, &group); err != nil {
		return nil, fmt.Errorf("failed to unmarshal hub group: %w", err)
	}

	return &group, nil
}

// SetGroupIdentity records the identity provider's group id and name on a Hub group.
//
// groupID must be the group's Hub id, which YouTrack reports as the group's ring id.
// The YouTrack group API silently drops idpGroupId from a create or update payload --
// it neither stores nor returns the field -- so tagging a group created through
// CreateGroup takes this second call against Hub.
//
// Hub answers this write with 200 and an empty body, so the result is read back rather
// than parsed from the response.
func (c *Client) SetGroupIdentity(ctx context.Context, groupID, idpGroupID, idpGroupName string) (*HubGroup, error) {
	if strings.TrimSpace(groupID) == "" {
		return nil, fmt.Errorf("group id must not be empty")
	}
	if strings.TrimSpace(idpGroupID) == "" {
		return nil, fmt.Errorf("idp group id must not be empty")
	}

	payload := HubGroup{IDPGroupID: idpGroupID, IDPGroupName: idpGroupName}
	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal group identity payload: %w", err)
	}

	values := url.Values{}
	values.Set("fields", hubGroupIdentityFields)

	endpoint := fmt.Sprintf("%s/%s/%s?%s", c.HostURL, hubRestUserGroupsAPIPath, url.PathEscape(groupID), values.Encode())
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create set group identity request: %w", err)
	}

	if _, err := c.doRequest(req); err != nil {
		return nil, fmt.Errorf("failed to set group identity: %w", err)
	}

	return c.GetHubGroup(ctx, groupID)
}
