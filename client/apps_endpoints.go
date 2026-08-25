package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// App HTTP handlers let an installed app expose its own REST endpoints. Unlike
// the app scoping endpoints in apps.go, these are part of the documented
// YouTrack API:
// https://www.jetbrains.com/help/youtrack/devportal/apps-reference-http-handlers.html
//
// YouTrack exposes a handler under the entity its scope refers to. Only the
// project scope is implemented here:
//
//	{host}/api/admin/projects/{projectID}/extensionEndpoints/{app}/{handler}/{path}
//
// where app is the app name from its manifest (for example "release-manager"),
// handler is the backend script filename without the .js extension (for example
// "backend" for backend.js), and path is the endpoint path the handler
// declares (for example "app-settings").
//
// The request and response bodies are entirely defined by the app, not by
// YouTrack, so they are passed through as raw JSON. Callers own the schema and
// are expected to model it on their side; keeping it opaque here avoids baking
// any single app's settings format into this client.
//
// Calling an endpoint requires the same permissions as its scope entity, so a
// project-scoped endpoint needs access to the target project.
//
// Verified against YouTrack 2026.2 with the Release Manager app (1.2.2).
// Two behaviours are worth knowing before writing through these endpoints,
// because they are properties of the app rather than of YouTrack:
//   - A handler that stores its settings as a single JSON blob typically
//     replaces the whole blob on write, so a partial payload silently drops
//     the keys it omits. Read the current value, overlay the fields you own,
//     and write the merged result back.
//   - A handler may validate its payload and reject values it considers
//     incomplete, including the empty state it reports before it is first
//     configured. Do not assume a value read from an endpoint can be written
//     back to it unchanged.

const (
	// projectExtensionEndpointsSubPath is the sub-resource under a project that
	// exposes the HTTP handlers of installed apps.
	projectExtensionEndpointsSubPath = "extensionEndpoints"

	// projectExtensionEndpointFmt renders
	// {host}/{projectsAPIPath}/{projectID}/extensionEndpoints/{app}/{handler}/{path}.
	projectExtensionEndpointFmt = "%s/%s/%s/%s/%s/%s/%s"

	errCreateAppEndpointRequest = "failed to create %s app endpoint request: %w"
	errCallAppEndpoint          = "failed to call %s app endpoint: %w"
)

// AppEndpointRef identifies a single HTTP handler endpoint exposed by an
// installed app.
//
// AppName is the app's manifest name (for example "release-manager"), Handler
// is the backend script filename without its .js extension (for example
// "backend"), and Path is the endpoint path declared by that handler (for
// example "app-settings").
type AppEndpointRef struct {
	AppName string
	Handler string
	Path    string
}

// endpointURL builds the project-scoped URL for the endpoint.
func (r AppEndpointRef) endpointURL(hostURL, projectID string) string {
	return fmt.Sprintf(projectExtensionEndpointFmt, hostURL, projectsAPIPath,
		url.PathEscape(projectID), projectExtensionEndpointsSubPath,
		url.PathEscape(r.AppName), url.PathEscape(r.Handler), url.PathEscape(r.Path))
}

// GetProjectAppEndpoint performs a GET against a project-scoped app HTTP
// handler endpoint and returns the raw response body.
//
// The response schema is defined by the app, so it is returned undecoded for
// the caller to unmarshal into its own type.
func (c *Client) GetProjectAppEndpoint(ctx context.Context, projectID string, ref AppEndpointRef) (json.RawMessage, error) {
	return c.callProjectAppEndpoint(ctx, httpMethodGet, projectID, ref, nil)
}

// PutProjectAppEndpoint performs a PUT against a project-scoped app HTTP
// handler endpoint, sending payload as the JSON request body, and returns the
// raw response body.
//
// Apps that store their configuration as a single JSON blob generally replace
// it wholesale, so payload should carry the complete desired state rather than
// only the fields being changed. See the note at the top of this file.
func (c *Client) PutProjectAppEndpoint(ctx context.Context, projectID string, ref AppEndpointRef, payload any) (json.RawMessage, error) {
	return c.callProjectAppEndpoint(ctx, httpMethodPut, projectID, ref, payload)
}

// PostProjectAppEndpoint performs a POST against a project-scoped app HTTP
// handler endpoint, sending payload as the JSON request body, and returns the
// raw response body.
func (c *Client) PostProjectAppEndpoint(ctx context.Context, projectID string, ref AppEndpointRef, payload any) (json.RawMessage, error) {
	return c.callProjectAppEndpoint(ctx, httpMethodPost, projectID, ref, payload)
}

// DeleteProjectAppEndpoint performs a DELETE against a project-scoped app HTTP
// handler endpoint. An endpoint that reports the entity as already gone is
// treated as success, so the call is idempotent.
func (c *Client) DeleteProjectAppEndpoint(ctx context.Context, projectID string, ref AppEndpointRef) error {
	if _, err := c.callProjectAppEndpoint(ctx, httpMethodDelete, projectID, ref, nil); err != nil && !IsNotFoundError(err) {
		return err
	}

	return nil
}

// callProjectAppEndpoint issues a request against a project-scoped app HTTP
// handler endpoint. A nil payload sends no request body.
func (c *Client) callProjectAppEndpoint(ctx context.Context, method, projectID string, ref AppEndpointRef, payload any) (json.RawMessage, error) {
	var body io.Reader

	if payload != nil {
		rb, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal %s app endpoint payload: %w", ref.Path, err)
		}

		body = bytes.NewReader(rb)
	}

	req, err := http.NewRequestWithContext(ctx, method, ref.endpointURL(c.HostURL, projectID), body)
	if err != nil {
		return nil, fmt.Errorf(errCreateAppEndpointRequest, ref.Path, err)
	}

	res, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf(errCallAppEndpoint, ref.Path, err)
	}

	return res, nil
}
