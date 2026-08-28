---
name: youtrack-api-integration
description: Use when adding, extending, or debugging support for a specific YouTrack REST API or Hub REST API resource/endpoint in this client (e.g. "add support for X", "the response is missing field Y", "how does the API represent Z"). Explains where to look up the endpoint in JetBrains' official API references, how to translate the docs into this repo's request/response types, and the recurring YouTrack/Hub quirks (fields param, $type discriminator, successor deletes, pagination, async processing). Pair with the youtrack-go-conventions skill for the Go implementation style.
---

# Mapping the YouTrack/Hub REST APIs onto this client

`youtrack-api-client` wraps two distinct but overlapping JetBrains REST APIs.
Before writing or changing a method, identify **which** API actually owns the
resource — guessing the URL shape from memory is how the fallback-endpoint
sprawl in `hub_users_groups.go`/`users_groups.go` happened.

## The two API references

- **YouTrack REST API** — issues, projects, custom fields, bundles, issue
  link types, workflows, articles, and most project-scoped admin resources.
  Reference: https://www.jetbrains.com/help/youtrack/devportal/rest-api-reference.html
- **Hub REST API** — the identity/authorization service YouTrack is built on:
  users, groups, roles, permissions, auth modules (OAuth2/SAML/LDAP), and
  global settings. Reference: https://www.jetbrains.com/help/youtrack/devportal/hub-rest-api-reference.html

Both references are large generated API docs organized by resource. When
implementing a new endpoint:

1. Fetch the relevant resource page from whichever reference above owns it
   (use WebFetch on the specific resource's URL — don't try to load the whole
   reference at once). Confirm the exact path, HTTP method, required/optional
   request fields, and the response schema's field names (YouTrack/Hub JSON
   is camelCase, e.g. `shortName`, `ringId`, `fromEmail`).
2. Check whether YouTrack exposes the same resource under its own `api/...`
   path as a proxy in front of Hub (common for users/groups/roles/permissions)
   — if so, both docs may describe overlapping behavior, and the proxy is
   sometimes incomplete or version-dependent. This repo already hit that with
   group membership and group deletion (see "Hub vs YouTrack duality" below).
3. Only then write the Go types and methods, following
   `youtrack-go-conventions` for the actual code shape.

If you're unsure which API a resource belongs to, a quick heuristic: if it's
about an *issue tracker concept* (project, issue, field, bundle, workflow),
it's YouTrack REST; if it's about *who can log in and what they're allowed to
do* (user, group, role, permission, auth module, license), it's Hub REST, even
when YouTrack also exposes it under `/api/...`.

## Recurring API quirks that shape this client's design

- **`fields` query parameter is mandatory on every request.** Both APIs
  return a minimal object (often just `id` and `$type`) unless you list every
  field you want via `?fields=...`. This is why every resource file in
  `client/` has a `xFieldsParam` constant spelling out the full field list,
  including nested object fields (`leader(id,login,name)`) and `$type` on
  every polymorphic sub-object. When adding a field to a Go struct, you must
  add it to the `fields` constant or the API will never return it.
- **`$type` discriminates polymorphic resources.** Custom field bundles,
  default values, and auth modules are polymorphic (`EnumBundle` vs
  `StateBundle`, `OAuth2AuthModule` vs other auth module kinds). Always
  request and preserve `$type` on these, and set it explicitly when
  constructing a payload to create/update one (see
  `CreateOAuth2AuthModule` setting `module.Type = oauth2AuthModuleType`
  before marshaling).
- **Hub vs YouTrack duality for identity resources.** Users, groups, and
  group membership can be reached through:
  - `api/users`, `api/groups`, `api/usergroups` (YouTrack's own REST surface,
    sometimes proxying to Hub, behavior varies by YouTrack version)
  - `hub/api/rest/users`, `hub/api/rest/usergroups` (Hub's REST surface
    directly)
  Different YouTrack/Hub version combinations support different subsets of
  these with different verbs (POST vs PUT) for the same logical operation.
  The client does **not** pick one and document a minimum version — it tries
  a prioritized list of endpoint variants and falls through on 404/405. See
  `AddUserToGroup` and `DeleteGroup` for the established pattern before adding
  a new membership-style call; extend the attempts list rather than picking
  a single URL.
- **Delete often requires a `successor`.** Hub enforces referential integrity
  on users/groups: deleting one requires a `successor` query parameter naming
  who/what inherits its memberships or ownership. Look for a `successor`
  parameter in the Hub REST docs for any new delete endpoint on an
  identity-ish resource before assuming a bare `DELETE /resource/{id}` works.
- **Pagination via `$top`/`$skip`.** List endpoints (`api/users`,
  `api/groups`, etc.) use OData-style `$top`/`$skip` query params, not
  `page`/`per_page`. Reuse `withPagination` (`users_groups.go`) for new list
  methods rather than reimplementing query building.
- **Some writes are asynchronously applied.** A handful of YouTrack
  operations return 2xx before the change is fully visible on a subsequent
  GET. Handle these with the `readBack*` helpers in `async.go`, which poll the
  read-back until the server reports the written value, bounded by the poll
  budget and by the caller's context (see `youtrack-go-conventions` for the
  projection pattern). Only add one if you've confirmed the operation actually
  behaves this way — don't add it speculatively. Never use `time.Sleep`.
- **List responses aren't always a bare array.** Some endpoints return a bare
  JSON array, others wrap it (`{"users": [...]}`, `{"usergroups": [...]}`)
  depending on YouTrack version/config. `ListUsers`/`GetUserByLogin` handle
  both shapes by trying to unmarshal into `[]Holder` first and falling back
  to the wrapped struct. Follow this pattern for any new list endpoint you
  discover behaves inconsistently across instances, but don't add it
  defensively where the response shape is actually stable — check real
  responses (via the docs' example payloads, or integration tests) first.

## Checking the models against YouTrack's own OpenAPI spec

Every instance publishes a generated OpenAPI 3.0 document at
`/api/openapi.json`. It is the fastest way to get a resource's real field list
before writing a struct, and to catch fields that drifted after a version bump.
It is also wrong in both directions often enough that every finding needs
confirming against a live instance. Use the `youtrack-openapi-drift` skill,
which has the script and the catalogue of known false positives — do not
generate code from the spec.

## Verifying against a real instance

If `YOUTRACK_BASE_URL`/`YOUTRACK_TOKEN` are available (see README.md
"Integration Tests"), prefer writing/running an integration test
(`TestIntegrationYouTrack...` or `TestIntegrationHub...`) against a real
instance to confirm field names and status codes rather than trusting the
docs' example payloads verbatim — JetBrains' reference examples occasionally
lag behind actual server behavior (this is exactly how the group-deletion and
membership fallback chains grew over time, per `CHANGELOG.md` 1.1.3–1.1.5).

## Workflow checklist for adding a new resource

1. Identify the owning API (YouTrack vs Hub) and fetch its reference page.
2. Note the path, verb(s), required/optional fields, and whether list
   endpoints support `$top`/`$skip`.
3. Add a new `client/<resource>.go` (or extend an existing domain file if the
   resource is a sub-resource, e.g. project custom fields live in
   `project.go`) with the const block, model/payload structs, and CRUD
   methods per `youtrack-go-conventions`.
4. Write table-driven unit tests in `client/<resource>_test.go` using
   `newTestClient`/`encodeJSON` from `roles_test.go`.
5. If the resource is identity-related, check whether it needs the
   Hub/YouTrack fallback pattern or a successor-on-delete parameter.
6. Update `CHANGELOG.md` and, if it's a headline feature, `README.md`.
7. Run `go vet ./...`, `golangci-lint run --config .golangci.yml`, and
   `go test ./client/...`.
