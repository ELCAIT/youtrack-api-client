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
