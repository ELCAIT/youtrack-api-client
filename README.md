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

## Integration Tests

Integration tests are opt-in and require a reachable YouTrack instance.

Set the following environment variables:

- `YOUTRACK_RUN_INTEGRATION_TESTS=1`
- `YOUTRACK_BASE_URL=https://your-youtrack.example.com`
- `YOUTRACK_TOKEN=perm:your-token`
- Optional: `YOUTRACK_RUN_HUB_INTEGRATION_TESTS=1` (enables Hub-style user/group membership lifecycle tests)
- Optional: `YOUTRACK_TEST_USER_PASSWORD=StrongPassword123!`

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
