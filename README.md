# youtrack-api-client

Go library to interact with the YouTrack REST API. It can be used to build
integrations such as Terraform providers, operators, or automation services.

This software is licensed under the Mozilla Public License 2.0 (MPL-2.0).
See the LICENSE file for details.

## Installation

```bash
go get github.com/elcait/youtrack-api-client
```

## Import

```go
import youtrack "github.com/elcait/youtrack-api-client/client"
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	youtrack "github.com/elcait/youtrack-api-client/client"
)

func main() {
	ctx := context.Background()

	client, err := youtrack.NewClient("https://your-youtrack.example.com", "perm:your-token")
	if err != nil {
		log.Fatalf("create client: %v", err)
	}

	user, err := client.GetUserByLogin(ctx, "admin")
	if err != nil {
		log.Fatalf("get user: %v", err)
	}

	log.Printf("Found user %s (%s)", user.Login, user.ID)
}
```

## Configuring the client

`NewClient` validates the host and token up front and accepts options, so a
client shared between goroutines is configured once and never mutated
afterwards:

```go
client, err := youtrack.NewClient(host, token,
	youtrack.WithUserAgent("my-operator/1.0.0"),   // identify your traffic in YouTrack's logs
	youtrack.WithTimeout(30*time.Second),          // per-request timeout
	youtrack.WithHTTPClient(customHTTPClient),     // custom transport, private CA, proxy
	youtrack.WithLogger(logger),                   // one *slog* line per request
)
```

A trailing slash on the host is accepted and normalised away, and a host that
is empty, relative, or not `http(s)` is rejected at construction rather than on
the first request.

## Error handling

Every call returns a typed `*HTTPError` for a server response, or a wrapped
transport error. Classify it with the predicates rather than by comparing
status codes — they see through `fmt.Errorf` wrapping:

| Predicate | Meaning |
| --- | --- |
| `IsNotFound(err)` | The entity does not exist. Prefer this over `IsNotFoundError`: it also covers collection lookups that found no match. |
| `IsAlreadyExists(err)` | A create collided with an entity that is already there. |
| `IsConflict(err)` | The write collided with the entity's current state; re-read and retry. |
| `IsUnauthorized(err)` / `IsForbidden(err)` | The token is invalid, or lacks a permission. Retrying will not help. |
| `IsRateLimited(err)` | The server applied a rate limit. |
| `IsRetryable(err)` | The failure may clear on its own: 429, 5xx, or a transport error. |
| `RetryAfter(err)` | The delay the server asked for, when it sent one. |

`HTTPError` carries YouTrack's own error payload (`ErrorCode`,
`ErrorDescription`) alongside the status code and raw body, which usually
explains a failure better than the status alone.

## Using this client from a Kubernetes operator

The distinction between *absent* and *unknown* is the one that matters. A
transport error read as absence makes a controller create duplicates, or
converge toward deleting live data:

```go
project, err := client.GetProject(ctx, id)
switch {
case err == nil:
	// Exists: converge its fields toward the desired state.
case youtrack.IsNotFound(err):
	// Absent: create it.
case youtrack.IsRetryable(err):
	// Unknown: requeue, changing nothing.
	if delay, ok := youtrack.RetryAfter(err); ok {
		return ctrl.Result{RequeueAfter: delay}, nil
	}
	return ctrl.Result{}, err
default:
	// Terminal: the request itself is wrong. Report it on the resource status
	// rather than retrying it forever.
	return ctrl.Result{}, reconcile.TerminalError(err)
}
```

Two further notes for controllers:

- **Reconcile against the whole collection.** Use the `ListAll*` methods
  (`ListAllProjects`, `ListAllUsers`, `ListAllGroups`, `ListAllYoutrackRoles`,
  `ListAllAssignedRoles`, `ListAllServices`, `ListAllApps`) rather than a single
  paginated `List*` call. Acting on only the first page makes a controller
  converge toward deleting everything it could not see.
- **Writes never block on a fixed sleep.** Several YouTrack settings endpoints
  acknowledge a write before applying it, so the matching `Update*` methods poll
  the read-back until it reflects the write, bounded by an internal timeout
  *and* by your context. They return as soon as the value converges and abort
  promptly on cancellation, so they neither stall a worker goroutine nor delay
  manager shutdown.

## Apps (undocumented API)

Apps can be enabled or disabled per project, or enabled for every project at once:

```go
// Name is the package-style identifier, not the UI display name (that is Title).
app, err := client.GetAppByName(ctx, "@jetbrains/youtrack-demo-client")
if err != nil {
	log.Fatalf("get app: %v", err)
}

// Attach the app to a project and enable it there (idempotent).
if _, err := client.EnableAppForProject(ctx, app.ID, projectID); err != nil {
	log.Fatalf("enable app: %v", err)
}

// Disable it again, leaving it attached to the project.
if _, err := client.DisableAppForProject(ctx, app.ID, projectID); err != nil {
	log.Fatalf("disable app: %v", err)
}

// Or enable it everywhere.
if _, err := client.EnableAppForAllProjects(ctx, app.ID); err != nil {
	log.Fatalf("enable app everywhere: %v", err)
}
```

> **Warning**
> The YouTrack endpoints behind these functions (`api/admin/apps` and its
> `usages` sub-resource) are **not part of the officially documented YouTrack
> REST API**. JetBrains support confirmed they work and that they are what the
> YouTrack UI itself uses, but also that they are undocumented and not
> guaranteed to stay backward compatible across YouTrack versions. Treat these
> functions as best-effort and verify them against your YouTrack version.
>
> The implementation was verified against a live YouTrack 2026.2 (build 18194)
> instance, and the opt-in acceptance suite below exists so that a version
> change breaking these endpoints shows up as a test failure. Note that the
> apps API silently ignores unknown names in a `fields` query instead of
> rejecting them, so a field that no longer exists comes back absent rather
> than as an error.

These calls require the *Update Project* permission on the target project. For
background on how apps are scoped globally versus per project, see the
[JetBrains documentation](https://www.jetbrains.com/help/youtrack/devportal/apps-global-project-level.html).

## App HTTP handler endpoints

Installed apps can expose their own REST endpoints, which is how an app's
per-project configuration is read and written. Unlike the scoping endpoints
above, these **are** part of the documented YouTrack API.

An endpoint is identified by the app's manifest name, the backend script
filename without its `.js` extension, and the path the handler declares:

```go
ref := youtrack.AppEndpointRef{
	AppName: "release-manager", // manifest name, not the display title
	Handler: "backend",         // backend.js
	Path:    "app-settings",    // path declared by the handler
}

raw, err := client.GetProjectAppEndpoint(ctx, projectID, ref)
if err != nil {
	log.Fatalf("read app settings: %v", err)
}

// The body is defined by the app, not by YouTrack, so the caller owns the schema.
var settings map[string]any
if err := json.Unmarshal(raw, &settings); err != nil {
	log.Fatalf("decode app settings: %v", err)
}

// Read, overlay only the fields you own, then write the merged result back.
settings["customFieldNames"] = []string{"State"}
if _, err := client.PutProjectAppEndpoint(ctx, projectID, ref, settings); err != nil {
	log.Fatalf("write app settings: %v", err)
}
```

These calls resolve to
`{host}/api/admin/projects/{projectID}/extensionEndpoints/{app}/{handler}/{path}`
and require the same permissions as the scope entity — for the project scope,
access to the target project. `PostProjectAppEndpoint` and
`DeleteProjectAppEndpoint` are available too; the latter treats an
already-absent entity as success.

> **Warning**
> The request and response bodies belong to the app, so the app decides how it
> handles them. Two behaviours are common enough to plan for:
>
> - **Writes usually replace the whole payload.** An app that stores its
>   settings as a single JSON blob will drop every key your payload omits.
>   Always read the current value, overlay the fields you own, and write the
>   merged result back — the read-modify-write above is the safe shape.
> - **Reads are not always writable back.** A handler may validate its payload
>   and reject values it considers incomplete, including the empty state it
>   reports before it has ever been configured. Do not assume a round-trip is
>   lossless.

Only the project scope is implemented. The API also defines global, issue,
article, and user scopes; see the
[JetBrains documentation](https://www.jetbrains.com/help/youtrack/devportal/apps-reference-http-handlers.html).

## Integration Tests

Integration tests are opt-in and require a reachable YouTrack instance.

Set the following environment variables:

- `YOUTRACK_RUN_INTEGRATION_TESTS=1`
- `YOUTRACK_BASE_URL=https://your-youtrack.example.com`
- `YOUTRACK_TOKEN=perm:your-token`
- Optional: `YOUTRACK_RUN_HUB_INTEGRATION_TESTS=1` (enables Hub-style user/group membership lifecycle tests)
- Optional: `YOUTRACK_TEST_USER_PASSWORD=StrongPassword123!`
- Optional: `YOUTRACK_RUN_APPS_INTEGRATION_TESTS=1` (enables the apps activation suite)
- Optional: `YOUTRACK_TEST_APP_NAME` (app to exercise in the apps suite; defaults to the first app reported)
- Optional: `YOUTRACK_TEST_PROJECT_LEADER` (leader for the throwaway project the apps suite creates; defaults to `admin`)
- Optional: `YOUTRACK_RUN_APPS_ALL_PROJECTS_TEST=1` (enables the `EnableAppForAllProjects` check)

Run only integration tests:

```bash
YOUTRACK_RUN_INTEGRATION_TESTS=1 \
YOUTRACK_BASE_URL="https://your-youtrack.example.com" \
YOUTRACK_TOKEN="perm:your-token" \
go test ./client -run TestIntegration -v
```

The integration suite is split into:

- YouTrack API suite (`TestIntegrationYouTrack...`): safe YouTrack resource lifecycle checks.
- Hub-dependent suite (`TestIntegrationHub...`): user lifecycle and group membership checks that rely on Hub semantics.
- Apps suite (`TestIntegrationYouTrackApp...`): activation/deactivation checks for the undocumented apps endpoints, behind their own switch.

The apps suite creates a throwaway project, runs the attach/enable/disable/
detach lifecycle against it, and deletes it again, so it leaves no trace on the
instance:

```bash
YOUTRACK_RUN_INTEGRATION_TESTS=1 \
YOUTRACK_RUN_APPS_INTEGRATION_TESTS=1 \
YOUTRACK_BASE_URL="https://your-youtrack.example.com" \
YOUTRACK_TOKEN="perm:your-token" \
go test ./client -run TestIntegrationYouTrackApp -v
```

`TestIntegrationYouTrackAppAllProjectsActivation` is gated separately by
`YOUTRACK_RUN_APPS_ALL_PROJECTS_TEST=1` because it enables the app in *every*
project on the instance. It captures the pre-existing usages and restores them
afterwards, but run it against a test instance rather than production.

Enable Hub-dependent tests only when your instance exposes/permits these operations:

```bash
YOUTRACK_RUN_INTEGRATION_TESTS=1 \
YOUTRACK_RUN_HUB_INTEGRATION_TESTS=1 \
YOUTRACK_BASE_URL="https://your-youtrack.example.com" \
YOUTRACK_TOKEN="perm:your-token" \
go test ./client -run TestIntegrationHub -v
```

If `YOUTRACK_TEST_USER_PASSWORD` is not set, the tests generate a strong default
password for created users.
