---
name: youtrack-go-conventions
description: Use whenever writing, reviewing, or refactoring Go code in this repository's client/ package (youtrack-api-client). Covers this project's architecture pattern (Client/doRequest, const blocks, CRUD method shape, error wrapping), lint/style rules enforced by golangci-lint and the Copilot instructions, and the table-driven httptest testing conventions. Trigger for any .go file change under client/, including adding new methods, models, or tests.
---

# Go conventions for youtrack-api-client

This repo is a single-package Go client (`package youtrack` in `client/`) that
wraps the YouTrack REST API and the Hub REST API for use by consumers such as
Terraform providers, operators, and automation services. There is no CLI, no
`main` package, no dependency injection framework — just a `Client` struct and
methods on it. Match the existing style exactly; do not introduce new
abstractions (interfaces, generics, builder patterns, service layers) that
aren't already used.

## Architecture, in one page

- `client/client.go` defines `Client{HostURL, HTTPClient, Token}`, `NewClient`,
  the shared `doRequest(req) ([]byte, error)` helper, and the `HTTPError` type
  with `IsNotFoundError`. Every API call goes through `doRequest`, which sets
  the `Authorization: Bearer <token>` and `Content-Type: application/json`
  headers and turns non-2xx responses into `*HTTPError`. Do not duplicate this
  logic — always build requests with `http.NewRequestWithContext` and pass
  them to `c.doRequest`.
- One file per resource/domain (`project.go`, `roles.go`,
  `users_groups.go`, `hub_users_groups.go`, `settings_*.go`, etc.), each with
  its own `const` block, model structs, and methods. Follow this layout for
  new resources instead of adding to an unrelated file.
- Shared string-format constants (`pathWithFieldsFormat = "%s/%s?%s"`,
  `specificIssueLinkType = "%s/%s/%s?%s"`) live wherever they were first
  defined and are reused across files. Grep for an existing format constant
  before adding a new one that does the same shape of URL.
- HTTP method/header/content-type constants (`httpMethodGet`, `httpMethodPost`,
  `httpMethodDelete`, `headerAuthorization`, `contentTypeJSON`, ...) live in
  `client.go`. Reuse them; do not hardcode `"GET"`/`"application/json"` again.

### The standard per-resource shape

```go
const (
    fooAPIPath     = "api/foos"
    fooFieldsParam = "fields=id,name,$type" // every field the caller may need, explicitly
)

// Foo represents a YouTrack foo.
type Foo struct {
    ID   string `json:"id,omitempty"`
    Name string `json:"name,omitempty"`
    Type string `json:"$type,omitempty"`
}

// FooCreatePayload is the request body for creating a foo.
type FooCreatePayload struct {
    Name string `json:"name"`
}

func (c *Client) CreateFoo(ctx context.Context, payload FooCreatePayload) (*Foo, error) {
    rb, err := json.Marshal(payload)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal foo: %w", err)
    }

    url := fmt.Sprintf(pathWithFieldsFormat, c.HostURL, fooAPIPath, fooFieldsParam)
    req, err := http.NewRequestWithContext(ctx, httpMethodPost, url, bytes.NewReader(rb))
    if err != nil {
        return nil, fmt.Errorf("failed to create foo request: %w", err)
    }

    body, err := c.doRequest(req)
    if err != nil {
        return nil, fmt.Errorf("failed to create foo: %w", err)
    }

    var created Foo
    if err := json.Unmarshal(body, &created); err != nil {
        return nil, fmt.Errorf("failed to unmarshal create foo response: %w", err)
    }

    return &created, nil
}
```

Naming: `CreateX`, `GetX`/`GetXByID`/`GetXByName`, `ListX` (with `top, skip int`
pagination via the existing `withPagination` helper in `users_groups.go` when
a list endpoint supports `$top`/`$skip`), `UpdateX`, `DeleteX`. Read methods
return `(*X, error)`; list methods return `([]X, error)`; delete methods
return `error` only.

When you add a `ListX(ctx, top, skip)`, also add a `ListAllX(ctx)` beside the
others in `pagination.go` — one line delegating to the generic `listAll`
helper. Reconciling callers need the whole collection: acting on only the first
page makes a controller converge toward deleting everything it could not see.

### Recurring quirks to preserve, not "fix"

- **`fields` is mandatory.** YouTrack/Hub only return the fields you ask for.
  Every request constant must spell out every field the model needs,
  including `$type` for polymorphic entities (bundles, custom field values,
  auth modules). If you add a field to a struct, add it to the matching
  `fields=...` constant too, or callers will silently get zero values.
- **Optional vs. absent in payloads.** Use pointer fields (`*bool`, `*string`,
  `*UserRef`) with `omitempty` in `*UpdatePayload`/`*UpsertPayload` structs
  whenever `false`/`""`/zero-value is a legitimate value that must still be
  sent (see `ProjectUpdatePayload.Archived *bool`,
  `ProjectCustomFieldUpsertPayload`). Plain `bool`/`string` with `omitempty`
  is fine only for full response models or "create" payloads where the field
  is genuinely never intentionally cleared.
- **Hub vs. YouTrack endpoint duality.** User/group/permission management is
  implemented by Hub but often proxied under `api/...` by YouTrack, and the
  proxy behavior differs across YouTrack versions. See
  `hub_users_groups.go` (`AddUserToGroup`) and `users_groups.go`
  (`DeleteGroup`) for the established pattern: build an ordered list of
  `(method, endpoint[, body])` attempts from most-canonical to
  legacy/compatibility, loop through them, treat 404/`IsNotFoundError` and 405
  (`http.StatusMethodNotAllowed`) as "try the next one"
  (`isRetryableMembershipEndpointError`), and only return an error if every
  attempt failed. Follow this pattern for any new membership-style endpoint
  instead of hardcoding a single URL.
- **Successor-on-delete.** Deleting a user or group requires a `successor`
  query parameter (the replacement entity that inherits members/ownership).
  Resolve a sane default successor (e.g. the `guest` user via
  `GetUserByLogin`, or the all-users group via `GetAllUsersGroup`) rather than
  requiring the caller to pass one, unless the caller-supplied ID is the
  whole point of the method (see `DeleteGroup(ctx, groupID, successorID)` vs.
  `DeleteUser(ctx, userID)`).
- **Idempotent deletes.** `DeleteUser`/`RemoveUserFromGroup` treat a 404 on
  delete as success (`IsNotFoundError(err)` → `return nil`). Preserve this for
  any new delete/remove method — callers should be able to call delete twice
  safely.
- **Async read-back, never a sleep.** Some YouTrack/Hub writes return 2xx
  before the change is visible to a subsequent GET. Handle this with
  `readBackEqual` / `readBackDeepEqual` / `readBackAfterWrite` (`async.go`),
  which re-read until the server reports the value that was written, bounded
  by `asyncPollTimeout` *and* by the caller's context. Project the entity down
  to the fields the write actually controls — see the `*State` types in
  `async_state.go` — so server-populated fields (IDs, secrets Hub never echoes
  back, computed state) don't prevent it from ever settling. Never add a
  `time.Sleep`: the old `waitForAsyncProcessing()` blocked every caller for a
  fixed delay and ignored cancellation, which stalls an operator's worker
  goroutines and delays manager shutdown. Only use a read-back where the
  endpoint genuinely behaves this way; don't add it defensively.
- **Case-insensitive / lookup-by-name.** When an API only supports listing,
  not filtering by name/login, filter client-side after listing
  (`GetUserByLogin`, `GetUserGroupByName` use `strings.EqualFold` for names).

## Style rules

Sourced from `.github/copilot-instructions.md` and
`.github/instructions/go.instructions.md`. Note: `.golangci.yml` enables
`gofmt`, `goimports`, `govet`, `errcheck`, `gosec`, `staticcheck`,
`copyloopvar`, `durationcheck`, `forcetypeassert`, `ineffassign`, `makezero`,
`misspell`, `nilerr`, `predeclared`, `unconvert`, `unparam`, `unused`, and
`revive`'s `exported` rule — but **not** `gocognit`/`cyclop` or
`dupl`/`goconst`. The complexity cap and the no-duplication rules below are
*not* mechanically gated by CI today; they only exist in the instruction
files, so review for them deliberately (and flag it if you're asked to relax
one, since there's no lint failure to fall back on).

- `gofmt`/`goimports` clean; run `go vet` and `golangci-lint run --config
  .golangci.yml` (or `./golangci-lint.sh`) before considering work done.
- Every exported identifier needs a doc comment starting with its name
  (`revive` `exported` rule is enabled).
- Always propagate `context.Context` as the first parameter of any function
  that makes a network call.
- Never ignore a returned error (`errcheck`); wrap with
  `fmt.Errorf("...: %w", err)`, never swallow or just log it.
- No global mutable state, no panics except truly unrecoverable programmer
  errors, no reflection unless unavoidable.
- **No string-literal duplication.** Extract any repeated string (path
  segment, `fields=...` query, error-format string) to a `const` — called out
  explicitly ("IMPORTANT: do not duplicate string literals") and matches
  existing code: every URL path, format string, and field list in `client/`
  is a named constant, never an inline literal repeated across methods.
- **Escape every ID interpolated into a path.** Identifiers reaching this
  client often come from user-authored configuration (a Kubernetes resource
  spec, Terraform HCL), so one containing `/`, `?`, or `#` must not be able to
  address a different endpoint. Use `c.buildURL` (`url.go`), which escapes
  segments and normalises separators, or `url.PathEscape` when extending an
  existing `fmt.Sprintf` site.
- **No duplicated logic.** "Do not duplicate code" / "Use helper functions
  where appropriate" — if you're about to write the same request-build/
  marshal/unmarshal sequence a second time (e.g. a second lookup-by-field
  method, a second membership-attempt loop), factor out a helper first. See
  `withPagination` (`users_groups.go`), `sendMembershipRequest` and
  `isRetryableMembershipEndpointError` (`hub_users_groups.go`) for the shape
  of helper this repo already extracts — small, package-private, named after
  what they do.
- **Cognitive complexity ≤ 15 per function.** Keep functions small; prefer
  early returns over nested conditionals, and prefer a data-driven loop
  (slice of attempts/cases) over a chain of `if`/`else if`. The
  membership-fallback loops in `hub_users_groups.go` are already near this
  ceiling — don't add more branching inside them; add another attempt entry
  to the slice instead. There's no `gocognit` lint gate for this today (see
  above), so this is a manual review criterion, not something `golangci-lint
  run` will catch.
- Max 7 parameters per function; bundle related fields into a payload struct
  instead of adding more parameters.
- `gosec` is enabled; the client package intentionally suppresses G107/G704
  (SSRF via user-configured `HostURL`) only in `client/` — don't add new
  `//nolint` suppressions elsewhere without a comment explaining why, matching
  the existing `//nolint:gosec` / `// #nosec G117` comments on secret fields
  (`ClientSecret`, `Password`) that must legitimately be marshaled.
- `copyloopvar`, `ineffassign`, `makezero`, `nilerr`, `predeclared`,
  `unconvert`, `unparam`, `unused`, `misspell`, `durationcheck` are all
  enabled — write straightforward, idiomatic Go and these won't fire.

## Testing conventions

- Unit tests are table-driven and use `httptest.NewServer` via the shared
  helpers defined in `client/roles_test.go`: `newTestClient(t, handler)
  (*Client, *httptest.Server)` and `encodeJSON(t, w, v)`. These helpers (and
  shared constants like `errExpectedError`, `fmtUnexpectedError`,
  `fmtUnexpectedID`) are package-private and available to every `_test.go`
  file in `client/` — reuse them, don't redefine equivalents in a new test
  file. `errUnexpectedMethod` lives in
  `settings_global_time_tracking_test.go` for the same reason.
- Structure: a `tests := []struct{ name string; ...; wantErr bool }{...}` slice,
  looped with `t.Run(tc.name, func(t *testing.T) { t.Parallel(); ... })`, and
  `t.Parallel()` at the top of the outer test function too.
- Assert on request shape (method, path, query params, body contents) inside
  the `httptest` handler, and assert on the returned value/error after the
  call — see `TestAddUserToGroupFallbackOnMethodNotAllowed` for how to assert
  a specific fallback sequence occurred.
- Integration tests live in `*_integration_test.go`, are named
  `TestIntegrationYouTrack...` or `TestIntegrationHub...`, and must be gated
  so they no-op unless the relevant env var is set (`YOUTRACK_RUN_INTEGRATION_TESTS=1`,
  optionally `YOUTRACK_RUN_HUB_INTEGRATION_TESTS=1`) — see the `README.md`
  "Integration Tests" section for the exact contract before adding new ones.
- Run `go test ./client/... -v` (unit tests only run by default; integration
  tests self-skip without the env vars).

## Release hygiene

- User-facing changes get an entry in `CHANGELOG.md` under an `## x.y.z`
  heading with `FEATURES:`/`IMPROVEMENTS:`/`BUG FIXES:` subsections (see the
  existing entries) — add to the top (unreleased) section rather than
  rewriting history.
- `README.md`'s Quick Start/Integration Tests sections should stay accurate if
  you change `NewClient`'s signature or the integration test env var contract.
