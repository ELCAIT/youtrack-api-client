// Package youtrack is a Go client for the YouTrack and Hub REST APIs.
//
// It covers the administrative surface of a YouTrack instance — projects and
// their custom fields, users, groups, roles and role assignments, custom field
// bundles, issue link types, authentication modules, Hub services, installed
// apps, and global settings — and is built for programs that reconcile that
// configuration, such as Kubernetes operators and Terraform providers.
//
// # Getting started
//
// Construct a Client with the instance URL and a permanent token, then call
// methods on it. Every method takes a context.Context as its first argument and
// honours its cancellation and deadline.
//
//	client, err := youtrack.NewClient("https://youtrack.example.com", "perm:token")
//	if err != nil {
//	    return err
//	}
//
//	project, err := client.GetProject(ctx, "0-1")
//
// A Client is safe for concurrent use by multiple goroutines. Configure it with
// Option values passed to NewClient rather than by assigning to its fields
// afterwards, so a client shared between workers stays race-free:
//
//	client, err := youtrack.NewClient(host, token,
//	    youtrack.WithUserAgent("my-operator/1.0.0"),
//	    youtrack.WithTimeout(30*time.Second),
//	    youtrack.WithLogger(logger),
//	)
//
// # Classifying errors
//
// Every call returns either a typed *HTTPError describing the server's response
// or a wrapped transport error. Rather than matching on status codes, use the
// predicates, which see through wrapping:
//
//   - [IsNotFound] — the entity does not exist. Prefer it over [IsNotFoundError]:
//     it also covers lookups that scanned a collection and found no match.
//   - [IsAlreadyExists] — a create collided with an entity that is already
//     there. See its documentation for the caveat on YouTrack's inconsistency.
//   - [IsConflict] — the write collided with the entity's current state; re-read
//     and build a new request.
//   - [IsUnauthorized], [IsForbidden] — the token is invalid, or lacks a
//     permission. Retrying will not help.
//   - [IsRetryable] — the failure may clear on its own: rate limiting, a 5xx, or
//     a transport error that leaves the outcome unknown.
//   - [RetryAfter] — the delay the server asked for, when it sent one.
//
// # Writing a reconcile loop
//
// The distinction between absence and failure is the one that matters most. A
// transport error must never be read as "the entity is gone", or a controller
// will create duplicates or converge toward deleting live data:
//
//	project, err := client.GetProject(ctx, id)
//	switch {
//	case err == nil:
//	    // Exists: converge its fields toward the desired state.
//	case youtrack.IsNotFound(err):
//	    // Absent: create it.
//	case youtrack.IsRetryable(err):
//	    // Unknown: requeue with backoff, changing nothing.
//	    if delay, ok := youtrack.RetryAfter(err); ok {
//	        return ctrl.Result{RequeueAfter: delay}, nil
//	    }
//	    return ctrl.Result{}, err
//	default:
//	    // Terminal: the request itself is wrong. Report it on the resource's
//	    // status instead of retrying it forever.
//	    return ctrl.Result{}, reconcile.TerminalError(err)
//	}
//
// When reconciling against a whole collection, use the ListAll methods
// ([Client.ListAllProjects], [Client.ListAllUsers], and their siblings) rather
// than a single paginated List call. Acting on only the first page makes a
// controller converge toward deleting everything it could not see.
//
// # Asynchronous writes
//
// Several YouTrack settings endpoints acknowledge a write before applying it,
// so a read issued immediately afterwards can still report the previous value.
// The Update methods for those endpoints poll the read-back until it reflects
// the write, bounded by an internal timeout and by the caller's context, and
// return the converged value. A caller does not need to sleep or re-read.
//
// # Pagination
//
// List methods take top and skip arguments mapping to YouTrack's $top and $skip
// parameters; passing 0 for both leaves the server's own default in place. The
// ListAll variants page to exhaustion.
//
// # Not everything here is documented by JetBrains
//
// The apps surface ([Client.ListApps], [Client.EnableAppForProject], and the
// rest of the AppUsage methods) calls endpoints that JetBrains has confirmed
// work but does not document or guarantee across versions. See the
// documentation on those methods before depending on them.
package youtrack
