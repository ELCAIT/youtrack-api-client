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

// The endpoints used in this file are NOT part of the officially documented
// YouTrack REST API. JetBrains support confirmed they work and that they are
// what the YouTrack UI itself uses, but also that they are not documented and
// not guaranteed to stay backward compatible across YouTrack versions. Treat
// every function here as best-effort and pin/verify your YouTrack version when
// relying on them.
//
// Reference for the app scoping model these endpoints expose:
// https://www.jetbrains.com/help/youtrack/devportal/apps-global-project-level.html
//
// Note that PUT on the usages collection is deliberately not implemented: it
// replaces the whole list of projects an app is attached to, so it can silently
// detach the app from projects it was not asked to touch. Attach, toggle, and
// detach are done one project at a time instead.
//
// All app usage calls require the Update Project permission on the target project.
//
// Verified against YouTrack 2026.2 (build 18194). Two behaviours of this API are
// worth knowing, because this client depends on both:
//   - Unknown names in a `fields` query are silently ignored rather than
//     rejected, so a field that does not exist comes back absent instead of
//     erroring. The `enabled`, `type`, and `author` fields quoted in some
//     support answers do not exist on the App entity and are therefore not
//     modelled here; whether an app is on or off is a per-project property of
//     its AppUsage, not of the App itself.
//   - Attaching an app to a project it is already attached to is idempotent
//     server-side: it returns the existing usage rather than creating a second
//     one or failing.

const (
	appsAPIPath      = "api/admin/apps"
	appUsagesSubPath = "usages"

	appFields      = "id,name,title,description,version,vendor(name,url,email)"
	appFieldsParam = "fields=" + appFields

	appUsageFields      = "id,enabled,project(id,name,shortName),$type"
	appUsageFieldsParam = "fields=" + appUsageFields

	appUsagesListFmt  = "%s/%s/%s/%s?%s"
	appUsageByIDFmt   = "%s/%s/%s/%s/%s?%s"
	appUsageDeleteFmt = "%s/%s/%s/%s/%s"

	// appLookupPageSize is the page size used when paging through apps or
	// projects to find a match by name.
	appLookupPageSize = 100

	errMarshalAppUsage = "failed to marshal app usage payload: %w"
)

// errAppNotFound is returned by GetAppByName when no app matches the name.
var errAppNotFound = fmt.Errorf("app %w", ErrNotFound)

// ListApps returns the apps installed on the instance and supports optional
// pagination via top/skip. Pass 0 for top and skip to use the default
// server-side pagination.
func (c *Client) ListApps(ctx context.Context, top, skip int) ([]App, error) {
	query := withPagination(appFields, top, skip)
	endpoint := fmt.Sprintf(pathWithFieldsFormat, c.HostURL, appsAPIPath, query)

	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list apps request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	var apps []App
	if err = json.Unmarshal(body, &apps); err != nil {
		return nil, fmt.Errorf("failed to unmarshal apps response: %w", err)
	}

	return apps, nil
}

// GetAppByID retrieves a single app by its entity ID (for example "145-92").
func (c *Client) GetAppByID(ctx context.Context, appID string) (*App, error) {
	endpoint := fmt.Sprintf(specificIssueLinkType, c.HostURL, appsAPIPath, url.PathEscape(appID), appFieldsParam)

	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get app request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	var app App
	if err = json.Unmarshal(body, &app); err != nil {
		return nil, fmt.Errorf("failed to unmarshal app response: %w", err)
	}

	return &app, nil
}

// IsAppNotFoundError checks whether an error indicates that an app could not be found by name.
// Use IsNotFound instead when the entity type does not matter.
func IsAppNotFoundError(err error) bool {
	return errors.Is(err, errAppNotFound)
}

// GetAppByName retrieves an app by its name, paging through all apps. An exact
// match wins over a case-insensitive one on any page. It returns an error
// wrapping ErrNotFound when no app matches; test for it with IsNotFound.
func (c *Client) GetAppByName(ctx context.Context, name string) (*App, error) {
	app, err := lookupByNamePaginated(ctx, appLookupPageSize, name, c.getAppPage, appName)
	if err != nil {
		return nil, err
	}
	if app != nil {
		return app, nil
	}

	return nil, entityNotFoundf(errAppNotFound, "app with name %q", name)
}

func (c *Client) getAppPage(ctx context.Context, skip int) ([]App, error) {
	return c.ListApps(ctx, appLookupPageSize, skip)
}

func appName(app App) string {
	return app.Name
}

// ListAppUsages returns every project the app is attached to, together with
// whether the app is currently enabled in each of them.
func (c *Client) ListAppUsages(ctx context.Context, appID string) ([]AppUsage, error) {
	endpoint := fmt.Sprintf(appUsagesListFmt, c.HostURL, appsAPIPath, url.PathEscape(appID), appUsagesSubPath, appUsageFieldsParam)

	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list app usages request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list app usages: %w", err)
	}

	var usages []AppUsage
	if err = json.Unmarshal(body, &usages); err != nil {
		return nil, fmt.Errorf("failed to unmarshal app usages response: %w", err)
	}

	return usages, nil
}

// GetAppUsageForProject returns the app's usage entry for a single project, or
// nil (with a nil error) when the app is not attached to that project.
func (c *Client) GetAppUsageForProject(ctx context.Context, appID, projectID string) (*AppUsage, error) {
	usages, err := c.ListAppUsages(ctx, appID)
	if err != nil {
		return nil, err
	}

	return findAppUsageForProject(usages, projectID), nil
}

func findAppUsageForProject(usages []AppUsage, projectID string) *AppUsage {
	for i := range usages {
		if usages[i].Project != nil && usages[i].Project.ID == projectID {
			return &usages[i]
		}
	}

	return nil
}

// AttachAppToProject attaches an app to a project, making it available there.
// Attaching an app that is already attached is left to the API; use
// EnableAppForProject for an idempotent attach-and-enable.
func (c *Client) AttachAppToProject(ctx context.Context, appID, projectID string) (*AppUsage, error) {
	rb, err := json.Marshal(appUsageAttachPayload{Project: &Project{ID: projectID}})
	if err != nil {
		return nil, fmt.Errorf(errMarshalAppUsage, err)
	}

	endpoint := fmt.Sprintf(appUsagesListFmt, c.HostURL, appsAPIPath, url.PathEscape(appID), appUsagesSubPath, appUsageFieldsParam)

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create attach app to project request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to attach app to project: %w", err)
	}

	return c.decodeAppUsageOrRefetch(ctx, body, appID, projectID, "attach app to project")
}

// SetAppUsageEnabled enables or disables an app in the project its usage entry
// points at, without detaching it. Use DetachAppFromProject to remove the app
// from the project entirely.
func (c *Client) SetAppUsageEnabled(ctx context.Context, appID, usageID string, enabled bool) (*AppUsage, error) {
	rb, err := json.Marshal(appUsageEnabledPayload{Enabled: enabled})
	if err != nil {
		return nil, fmt.Errorf(errMarshalAppUsage, err)
	}

	endpoint := fmt.Sprintf(appUsageByIDFmt, c.HostURL, appsAPIPath, url.PathEscape(appID),
		appUsagesSubPath, url.PathEscape(usageID), appUsageFieldsParam)

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create set app usage enabled request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to set app usage enabled: %w", err)
	}

	var usage AppUsage
	if err = json.Unmarshal(body, &usage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal app usage response: %w", err)
	}

	return &usage, nil
}

// DeleteAppUsage detaches an app from a project by usage ID. A usage that no
// longer exists is treated as success, so the call is idempotent.
func (c *Client) DeleteAppUsage(ctx context.Context, appID, usageID string) error {
	endpoint := fmt.Sprintf(appUsageDeleteFmt, c.HostURL, appsAPIPath, url.PathEscape(appID),
		appUsagesSubPath, url.PathEscape(usageID))

	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete app usage request: %w", err)
	}

	if _, err = c.doRequest(req); err != nil && !IsNotFoundError(err) {
		return fmt.Errorf("failed to delete app usage: %w", err)
	}

	return nil
}

// DetachAppFromProject removes an app from a project entirely. An app that is
// not attached to the project is treated as success, so the call is idempotent.
func (c *Client) DetachAppFromProject(ctx context.Context, appID, projectID string) error {
	usage, err := c.GetAppUsageForProject(ctx, appID, projectID)
	if err != nil {
		return err
	}
	if usage == nil {
		return nil
	}

	return c.DeleteAppUsage(ctx, appID, usage.ID)
}

// EnableAppForProject makes an app available and enabled in a project. It
// attaches the app first when it is not attached yet, and enables an existing
// usage that is currently disabled. Calling it on an app that is already
// enabled for the project is a no-op.
func (c *Client) EnableAppForProject(ctx context.Context, appID, projectID string) (*AppUsage, error) {
	usage, err := c.GetAppUsageForProject(ctx, appID, projectID)
	if err != nil {
		return nil, err
	}

	if usage == nil {
		attached, err := c.AttachAppToProject(ctx, appID, projectID)
		if err != nil {
			return nil, err
		}
		if attached.Enabled {
			return attached, nil
		}
		usage = attached
	} else if usage.Enabled {
		return usage, nil
	}

	return c.SetAppUsageEnabled(ctx, appID, usage.ID, true)
}

// DisableAppForProject disables an app in a project while leaving it attached.
// It returns nil (with a nil error) when the app is not attached to the project
// at all, since there is then nothing to disable.
func (c *Client) DisableAppForProject(ctx context.Context, appID, projectID string) (*AppUsage, error) {
	usage, err := c.GetAppUsageForProject(ctx, appID, projectID)
	if err != nil {
		return nil, err
	}
	if usage == nil {
		return nil, nil
	}
	if !usage.Enabled {
		return usage, nil
	}

	return c.SetAppUsageEnabled(ctx, appID, usage.ID, false)
}

// EnableAppForAllProjects enables an app in every project on the instance,
// attaching it where needed. Projects are enumerated with ListProjects, so
// archived and template projects are included as well.
//
// There is no single API call for this: the app is enabled one project at a
// time. On the first failure the usages enabled so far are returned alongside
// the error, so the caller can see how far it got.
func (c *Client) EnableAppForAllProjects(ctx context.Context, appID string) ([]AppUsage, error) {
	projects, err := c.listAllProjects(ctx)
	if err != nil {
		return nil, err
	}

	usages := make([]AppUsage, 0, len(projects))
	for i := range projects {
		usage, err := c.EnableAppForProject(ctx, appID, projects[i].ID)
		if err != nil {
			return usages, fmt.Errorf("failed to enable app for project '%s': %w", projects[i].ID, err)
		}
		usages = append(usages, *usage)
	}

	return usages, nil
}

// listAllProjects pages through every project on the instance.
func (c *Client) listAllProjects(ctx context.Context) ([]Project, error) {
	var all []Project

	for skip := 0; ; skip += appLookupPageSize {
		page, err := c.ListProjects(ctx, appLookupPageSize, skip)
		if err != nil {
			return nil, err
		}

		all = append(all, page...)

		if len(page) < appLookupPageSize {
			return all, nil
		}
	}
}

// decodeAppUsageOrRefetch decodes an app usage response body, falling back to a
// lookup in the usages list when the API answers with an empty body instead of
// the created entity.
func (c *Client) decodeAppUsageOrRefetch(ctx context.Context, body []byte, appID, projectID, action string) (*AppUsage, error) {
	if len(bytes.TrimSpace(body)) > 0 {
		var usage AppUsage
		if err := json.Unmarshal(body, &usage); err != nil {
			return nil, fmt.Errorf("failed to unmarshal app usage response: %w", err)
		}
		if usage.ID != "" {
			return &usage, nil
		}
	}

	usage, err := c.GetAppUsageForProject(ctx, appID, projectID)
	if err != nil {
		return nil, err
	}
	if usage == nil {
		return nil, fmt.Errorf("failed to %s: no app usage found for project '%s' after the call succeeded", action, projectID)
	}

	return usage, nil
}
