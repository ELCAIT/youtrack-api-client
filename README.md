# youtrack-api-client

Go library to interact with the YouTrack REST API. It can be used to build
integrations such as Terraform providers, operators, or automation services.

## Installation

```bash
go get github.com/sflotron/youtrack-api-client
```

## Import

```go
import youtrack "github.com/sflotron/youtrack-api-client/client"
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	youtrack "github.com/sflotron/youtrack-api-client/client"
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

## License

This project is licensed under GNU GPLv3. See the LICENSE file for details.
