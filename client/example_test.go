package youtrack_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	youtrack "github.com/elcait/youtrack-api-client/client"
)

func ExampleNewClient() {
	client, err := youtrack.NewClient("https://youtrack.example.com", "perm:token")
	if err != nil {
		log.Fatal(err)
	}

	_ = client
}

// Options configure the client at construction time, which is what keeps a
// client shared between goroutines race-free.
func ExampleNewClient_options() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	client, err := youtrack.NewClient("https://youtrack.example.com", "perm:token",
		youtrack.WithUserAgent("my-operator/1.0.0"),
		youtrack.WithTimeout(30*time.Second),
		youtrack.WithLogger(logger),
	)
	if err != nil {
		log.Fatal(err)
	}

	_ = client
}

// Distinguishing absence from failure is the central concern of a reconciling
// caller: a transport error read as absence leads to duplicate creates, or to
// deleting live data.
func ExampleIsNotFound() {
	client, err := youtrack.NewClient("https://youtrack.example.com", "perm:token")
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	project, err := client.GetProject(ctx, "0-1")
	switch {
	case err == nil:
		fmt.Println("found", project.Name)
	case youtrack.IsNotFound(err):
		fmt.Println("absent: safe to create")
	default:
		fmt.Println("unknown: retry, change nothing")
	}
}

// IsRetryable separates failures worth requeuing from those that will never
// succeed on their own.
func ExampleIsRetryable() {
	client, err := youtrack.NewClient("https://youtrack.example.com", "perm:token")
	if err != nil {
		log.Fatal(err)
	}

	_, err = client.ListAllProjects(context.Background())
	if err == nil {
		return
	}

	switch {
	case youtrack.IsRetryable(err):
		if delay, ok := youtrack.RetryAfter(err); ok {
			fmt.Printf("retry after %v\n", delay)

			return
		}
		fmt.Println("retry with backoff")
	case youtrack.IsForbidden(err):
		fmt.Println("token lacks a permission; report it, do not retry")
	default:
		fmt.Println("terminal; report it on the resource status")
	}
}

// The typed error carries YouTrack's own error payload, which usually explains
// the failure better than the status code does.
func ExampleHTTPError() {
	client, err := youtrack.NewClient("https://youtrack.example.com", "perm:token")
	if err != nil {
		log.Fatal(err)
	}

	_, err = client.GetProject(context.Background(), "0-1")

	var httpErr *youtrack.HTTPError
	if errors.As(err, &httpErr) {
		fmt.Println("status:", httpErr.StatusCode)
		fmt.Println("code:", httpErr.ErrorCode)
		fmt.Println("detail:", httpErr.ErrorDescription)
	}
}

// A reconcile that compares desired against actual state needs the whole
// collection; acting on one page converges toward deleting what it cannot see.
func ExampleClient_ListAllProjects() {
	client, err := youtrack.NewClient("https://youtrack.example.com", "perm:token")
	if err != nil {
		log.Fatal(err)
	}

	projects, err := client.ListAllProjects(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	for _, project := range projects {
		fmt.Println(project.ShortName, project.Name)
	}
}
