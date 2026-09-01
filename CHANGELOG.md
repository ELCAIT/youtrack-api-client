## 1.6.0
FEATURES:
- Add support for a Hub group's **identity-provider attributes**, so a group can be
  reconciled by the id of the role it represents at an upstream directory:
  - `FindGroupByIDPGroupID(ctx, authModuleName, idpGroupID)` returns the group carrying
    that id, or `nil, nil` when none does. Every provider writes into the same
    `idpGroupId` field, so the module name restricts the match to one provider's groups;
    passing an empty name matches on the id alone, which is only safe on an instance
    where a single provider tags groups.
  - `SetGroupIdentity(ctx, groupID, idpGroupID, idpGroupName)` records the attributes on
    an existing group. `groupID` is the group's Hub id, which YouTrack reports as its
    ring id.
  - `ListHubGroups(ctx, top, skip)` and `GetHubGroup(ctx, groupID)` read groups with
    those attributes, which the YouTrack-side group calls do not return.
  - `HubGroup` models the wire shape, and `HubGroup.AuthModuleName()` reports the
    provider a tagged group belongs to (Hub sets either `importedFromAuthModule` or
    `mappedInAuthModule` depending on how the association was made, so both are read).

  `idpGroupId` is a Hub-only field: the YouTrack group API neither returns it nor stores
  it, and **silently drops it** from a create or update payload rather than rejecting the
  request — verified against a live instance, where a group created through `CreateGroup`
  with the field set came back without it. Tagging a group therefore takes a second call
  against Hub, which answers it with `200` and an empty body, so `SetGroupIdentity` reads
  the group back rather than parsing the response.

  `SetGroupIdentity` takes the auth module id as well, because writing `idpGroupId`
  alone leaves the group associated with no module at all: unlike a user detail, which
  belongs to the module that created it, a group's association is a field of its own that
  Hub does not infer. Verified live — a group tagged without it was invisible to a
  module-filtered lookup, so a redelivered role event re-tagged instead of settling and a
  renamed role created a second group.

  Hub cannot filter on the attribute either — `idpGroupId` is not one of the fields its
  group query understands — so `FindGroupByIDPGroupID` scans the listing.

  `GetHubGroup` rejects a response whose id is not the one requested. Hub answers a
  request for an id it does not know with `200` and **some other group's record** rather
  than a `404`: on a live instance a deleted group's id returned an unrelated group, so a
  caller that trusted the status would read and then write the wrong group.

  Note for callers that delete groups: deletion goes through the **YouTrack** endpoint
  (`DeleteGroup`, which needs a real successor group id), never Hub's `usergroups`. A
  group lives on both sides, and removing only the Hub record leaves the YouTrack row
  behind in a state neither API will then delete.

- Add support for Hub **user details**, the per-user authentication identities that record who an account is at each external identity provider. This is what lets a caller reconcile an upstream directory's events against YouTrack accounts by the upstream's own user id:
  - `ListAuthModules(ctx)` and `GetAuthModuleByName(ctx, name)` enumerate and resolve the configured authentication modules. The name is what an operator recognises but it is instance-specific and renameable, so a caller should resolve it to an id once at startup and fail loudly if it is missing, rather than silently provisioning nothing on every later call.
  - `FindUserByAuthIdentifier(ctx, authModuleName, identifier)` returns the Hub user whose detail for that module carries the identifier, or `nil, nil` when no user matches. Absence is not an error: an event naming an account the instance does not have is a normal outcome, and a caller distinguishes it from a failure by the nil user.
  - `AddUserDetail(ctx, userID, detail)`, `ListUserDetails(ctx, userID)`, and `RemoveUserDetail(ctx, userID, detailID)` manage the details on one user. `RemoveUserDetail` treats an already-absent detail as success, like the other `Remove`/`Delete` methods in the client.
  - `UserDetail`, `AuthModule`, and `HubUser` model the wire shapes, and `Oauth2DetailsType` is the discriminator Hub uses for details produced by an OAuth2 or OIDC module. `HubUser` is deliberately a different projection from `User`: the authentication details live on the Hub side of the API, so a lookup by external identity returns the Hub shape.

  `AddUserDetail` is the part that does not merely read what Hub already knows. Hub writes a detail on first login, so an account provisioned from an upstream directory but never logged into carries no external identity at all and cannot be found by its upstream id — for a customer-facing instance that is the majority of accounts. Writing the detail at creation closes that gap, and when the user does eventually log in Hub matches them onto the existing detail instead of provisioning a second account.

  `FindUserByAuthIdentifier` narrows the listing to one module (`query=authModule: {name}`) and matches the identifier client-side, because Hub's user query understands the module but not the detail's identifier: the documented `authLogin` query field matches only a Hub-local login, and `identifier` is not a query field at all. The scan is therefore bounded by one module's population rather than by the whole user base — on the instance this was verified against, 4 users versus 1881 for the largest module. The module is checked alongside the identifier when matching, because a user has one detail per module and their identifiers are unrelated namespaces: matching on the value alone could pick up an unrelated provider's id.

## 1.5.0
FEATURES:
- Add an error taxonomy so callers can decide what to do with a failure without comparing status codes. All predicates unwrap, so they work on an error that has been wrapped with `fmt.Errorf`:
- `IsRetryable(err)` is true for rate limiting (429), the transient server statuses (500, 502, 503, 504), and transport-level failures — timeouts, refused connections, cancelled contexts — which leave the outcome of the call unknown. It is false for 400, 401, 403, 404, 405, 409, and 422, where retrying the identical request only repeats the failure. The 409 exclusion is deliberate: a conflict means the caller's view of the entity is stale, so the fix is to re-read and build a new request, not to resend this one.
- `IsConflict(err)`, `IsAlreadyExists(err)`, `IsUnauthorized(err)`, `IsForbidden(err)`, and `IsRateLimited(err)` name the individual cases at the call site.
- `RetryAfter(err)` returns the delay the server asked for and whether one was given, so a controller can pass it straight to a requeue rather than guessing at a backoff.
- `HTTPError` now carries YouTrack's own error payload in `ErrorCode` and `ErrorDescription`, parsed from both the `error_description` and `error_developer_message` spellings the APIs use, and `RetryAfter` from the header (accepting both the delay-seconds and HTTP-date forms). The raw `Body` is still populated, including when the payload is not a recognisable YouTrack error, so nothing is lost.
- Add `Option` arguments to `NewClient`, so a client that is shared between goroutines is configured once at construction rather than by assigning to its exported fields afterwards:
- `WithHTTPClient` supplies a custom `*http.Client`, which is how a private CA bundle, a proxy, or instrumentation gets in.
- `WithTimeout` sets the per-request timeout; a shorter context deadline still wins.
- `WithUserAgent` overrides the `User-Agent`, so an operator's traffic can be told apart from the UI's in server-side logs.
- `WithLogger` attaches an `*slog.Logger` that records one debug line per request (method, URL, status, duration) and one warning per failure. It never logs the token or the request and response bodies, because both routinely carry secrets. A logr-backed handler routes this into a controller-runtime log stream.
- Add `ListAll*` methods that page a collection to exhaustion: `ListAllProjects`, `ListAllUsers`, `ListAllGroups`, `ListAllYoutrackRoles`, `ListAllAssignedRoles`, `ListAllServices`, and `ListAllApps`. A reconciling caller needs the complete collection to decide what to create, update, and delete; reading only the first page makes a controller converge toward deleting everything it could not see, and writing that loop at each call site invites exactly that bug. The walk stops with an error rather than looping forever if the server ignores `$skip`.
- Add package documentation (`doc.go`) and runnable examples covering client construction, the options, error classification, and the reconcile switch. These are the pkg.go.dev front page, which the package previously did not have.

BUG FIXES:
- Writes to the settings endpoints that YouTrack applies asynchronously no longer block on a fixed `time.Sleep`. `waitForAsyncProcessing` slept 200ms after every such write, whether or not the change had landed, and ignored the caller's context entirely — so it stalled a controller's worker goroutines, delayed manager shutdown, and could still return a stale value under load. The ten affected methods (`UpdateGlobalSettings`, `UpdateLocaleSettings`, `UpdateRestSettings`, `UpdateBackupSettings`, `UpdateAppearanceSettings`, `UpdateWorkTimeSettings`, `UpdateService`, `UpdateYoutrackRole`, `UpdateOAuth2AuthModule`, `UpdateAzureAuthModule`) now poll the read-back until it reflects the write, return immediately once it does, and abort as soon as the context is cancelled. A write that never converges within the poll budget returns the last observed value rather than an error, because the write itself was already acknowledged by the server. `asyncPollTimeout` was declared but never used — the poll loop it was written for did not exist — and now bounds the wait.
- Entity IDs are escaped before being interpolated into a request path, across `project.go`, `users_groups.go`, `hub_users_groups.go`, `issue_links_types.go`, `customfields.go`, `bundle_state.go`, and `roles.go`. `url.PathEscape` was applied in some files and not others, so an identifier taken from user-authored configuration — a Kubernetes resource spec, for example — that contained `/`, `?`, or `#` produced a malformed URL that silently addressed a different endpoint instead of a clean error.
- `NewClient` validates its arguments instead of always returning a nil error. An empty, relative, or non-`http(s)` host and an empty token are now rejected at construction rather than surfacing as a confusing request failure later, and a trailing slash on the host is normalised away instead of producing a doubled separator in every path.
- Add `Service.Immutable`, which reports whether a Hub service is built in and therefore cannot be modified or deleted. Found by diffing against the **Hub** OpenAPI spec (`/hub/api/rest/openapi.json`), which covers the identity half of this client that the YouTrack spec does not. On a stock 2026.2 instance four of the five services ("YouTrack", "YouTrack Administration", "YouTrack Mobile", "Konnector") report `true`. The service field list now requests `immutable`.
- Add `Role.Immutable`, which reports whether a role is built into YouTrack and therefore cannot be modified or deleted. A reconciling caller should check it before attempting an update: on a 2026.2 instance six of the nine default roles report `true`, and writes to them fail. The role field list now requests `immutable`, without which the field would never populate.

IMPROVEMENTS:
- `Role.Key` is deprecated. Verified against a live YouTrack 2026.2 instance and against the OpenAPI schema: `Role` has no `key` field — it is absent from the schema, silently ignored when sent on a create, and never returned on a read even when requested explicitly, because YouTrack drops unknown names from a `fields` query rather than rejecting them. The field is documented and retained for backward compatibility and will be removed in the next major version. `Permission.Key` is unaffected: it is a real field and is populated.
- The role field list no longer requests `key` for roles, which the server was silently dropping on every request.
- The default HTTP transport raises `MaxIdleConnsPerHost` from Go's default of 2 to 10. A client owned by an operator is long-lived and talks to exactly one host, so the stock cap forced repeated TCP and TLS handshakes under concurrent reconciles.
- Every request now sends a `User-Agent` (`elcait-youtrack-api-client/<version>` by default), which the client previously omitted entirely.
- Renamed the `specificIssueLinkType` constant to `resourceByIDWithFieldsFormat`. It is the generic "entity by ID with a fields query" format and was already shared by projects and apps, so its name described only its first caller. This is an unexported constant, so no caller is affected.
- Added a URL builder (`buildURL`) that escapes path segments and normalises separators, plus `fieldsQuery`/`paginatedQuery` helpers, so new endpoints get correct escaping by default rather than by remembering to call `url.PathEscape`.
- `go.mod` no longer requires the test tooling as module dependencies. `gotestsum` and its transitive packages were listed as `// indirect` requirements even though nothing in the module imports them, which put them in the dependency graph of every downstream consumer; CI installs the tool directly with `go install`. The module now declares no dependencies at all, and the empty `go.sum` was removed.
- Test coverage rose from 55.7% to 60.9%, with new suites for the error taxonomy, the client options, the async read-back (including that a settled read does not wait and that cancellation aborts promptly), URL escaping and host normalisation, and the pagination walk.

## 1.4.0
FEATURES:
- Add support for calling the HTTP handler endpoints that installed apps expose, so an app's own configuration can be read and written from Go. Unlike the app scoping endpoints added in 1.3.0, these are part of the **documented** YouTrack API:
  - `GetProjectAppEndpoint(ctx, projectID, ref)`, `PutProjectAppEndpoint(ctx, projectID, ref, payload)`, `PostProjectAppEndpoint(ctx, projectID, ref, payload)`, and `DeleteProjectAppEndpoint(ctx, projectID, ref)` issue requests against `{host}/api/admin/projects/{projectID}/extensionEndpoints/{app}/{handler}/{path}`. `DeleteProjectAppEndpoint` treats an already-absent entity as success, like the other `Delete` methods in the client.
  - `AppEndpointRef{AppName, Handler, Path}` identifies the endpoint: the app's manifest name (for example `release-manager`), the backend script filename without its `.js` extension (for example `backend`), and the path the handler declares (for example `app-settings`).
  - Request and response bodies are defined by the app rather than by YouTrack, so they are passed through as raw JSON (`json.RawMessage`) and the caller owns the schema. This keeps any single app's settings format out of the client.
  - Only the project scope is implemented. The API also defines global, issue, article, and user scopes; they can be added the same way when a caller needs them.

IMPROVEMENTS:
- The app HTTP handler endpoints were verified against a live YouTrack 2026.2 instance with the Release Manager app (1.2.2). Two behaviours found there are documented in `client/apps_endpoints.go`, because they are properties of the app rather than of YouTrack and they shape how callers must write: a handler that stores its settings as one JSON blob replaces the whole blob on write, so a partial payload silently drops the keys it omits (read, overlay the fields you own, write the merged result back); and a handler may reject values it considers incomplete, including the empty state it reports before it is first configured, so a value read from an endpoint cannot be assumed writable back unchanged.

## 1.3.0
FEATURES:
- Add app management support, so an app can be enabled or disabled per project, or enabled for every project at once (issue #29):
  - `ListApps(ctx, top, skip)`, `GetAppByID(ctx, appID)`, and `GetAppByName(ctx, name)` to discover installed apps. `Name` is the package-style identifier (for example `@jetbrains/youtrack-demo-client`) and is what `GetAppByName` matches on; `Title` is the display name shown in the UI.
  - `ListAppUsages(ctx, appID)` and `GetAppUsageForProject(ctx, appID, projectID)` to read which projects an app is attached to and whether it is enabled in each of them. `GetAppUsageForProject` returns `nil` (with a `nil` error) when the app is not attached to the project.
  - `EnableAppForProject(ctx, appID, projectID)` and `DisableAppForProject(ctx, appID, projectID)` as idempotent, high-level toggles: enabling attaches the app first when needed, disabling leaves it attached.
  - `EnableAppForAllProjects(ctx, appID)` to enable an app in every project on the instance. There is no single API call for this, so projects are enumerated and handled one at a time; on the first failure the usages enabled so far are returned alongside the error.
  - `AttachAppToProject`, `SetAppUsageEnabled`, `DetachAppFromProject`, and `DeleteAppUsage` as the lower-level building blocks. `DetachAppFromProject` and `DeleteAppUsage` treat an already-absent usage as success, like the other `Delete`/`Remove` methods in the client.
- Add `ListProjects(ctx, top, skip)` to page through the projects on an instance, matching the pagination convention used by `ListUsers`/`ListServices`. It backs `EnableAppForAllProjects` and fills the gap left by the existing single-project `GetProject`.
- Add an opt-in acceptance suite for the apps endpoints, so regressions in this undocumented API surface as test failures rather than at runtime:
  - `TestIntegrationYouTrackAppProjectActivation` walks the full lifecycle (read by id/name, attach, enable, re-enable, disable, re-disable, re-enable, detach, re-detach) against a throwaway project it creates and deletes, asserting idempotency at every step.
  - `TestIntegrationYouTrackAppAllProjectsActivation` covers `EnableAppForAllProjects`. It captures the pre-existing usages, restores them afterwards, and sits behind its own switch because it touches every project on the instance.
  - New environment variables: `YOUTRACK_RUN_APPS_INTEGRATION_TESTS=1` (required for the suite), `YOUTRACK_RUN_APPS_ALL_PROJECTS_TEST=1` (all-projects test), `YOUTRACK_TEST_APP_NAME` (pick the app to exercise; defaults to the first app reported), and `YOUTRACK_TEST_PROJECT_LEADER` (leader for the throwaway project; defaults to `admin`).
- Add `ListYoutrackRoles(ctx, top, skip)` and `GetYoutrackRoleByName(ctx, name)` to look up roles by name. Roles could previously only be fetched by ID, so callers who knew a role by the name shown in the UI and in configuration (for example `ELCA Reader`) had no way to reach the ID that `AssignedRole` requires. `GetYoutrackRoleByName` pages through all roles and prefers an exact match over a case-insensitive one, matching `GetAppByName`; on a miss it returns an error wrapping `ErrNotFound`, and `IsRoleNotFoundError` narrows the check to roles.

IMPROVEMENTS:
- The YouTrack apps endpoints are **not part of the officially documented REST API**. JetBrains support confirmed they work and are what the YouTrack UI itself uses, but also that they are undocumented and not guaranteed to stay backward compatible across YouTrack versions. This is called out in the package documentation of `client/apps.go` and in the README; treat these functions as best-effort and verify them against your YouTrack version.
- The whole apps surface was verified against a live YouTrack 2026.2 (build 18194) instance rather than written from the endpoint sketch alone. Two behaviours found there are documented in `client/apps.go` because the client depends on them: unknown names in a `fields` query are silently ignored rather than rejected, and attaching an app to a project it is already attached to is idempotent server-side (it returns the existing usage instead of creating a second one).
- `PUT` on the app `usages` collection is deliberately not implemented: it replaces the entire list of projects an app is attached to, so it can silently detach the app from projects the caller never asked to touch. Attach, toggle, and detach act on one project at a time instead.
- Not-found reporting is now consistent across the client. Lookups that scan a collection previously each signalled absence differently — `GetUserGroupByName` returned a bare `fmt.Errorf`, `GetAppByName` an unexported sentinel — so no single predicate covered them. They now all wrap the exported `ErrNotFound` sentinel, and the new `IsNotFound` reports true both for that sentinel and for a 404 response. `IsNotFoundError` is unchanged and still matches only a 404, so existing callers keep their current behaviour; prefer `IsNotFound` in new code. This matters for reconciling callers: a transport failure must not read as absence, or the caller creates duplicates or converges toward deleting live data. `GetAppUsageForProject` remains the deliberate exception: it reports an unattached app as a `nil` result with a `nil` error, so a non-nil error from it always means the answer could not be determined.
- Extracted `roleFields` from `roleFieldsQueryParam` so the role field list can be reused with `withPagination`, matching the existing `appFields`/`appFieldsParam` convention.
- `App` models the fields the entity actually exposes — `ID`, `Name`, `Title`, `Description`, `Version`, and `Vendor` (an `AppVendor` with `Name`/`URL`/`Email`). The `enabled`, `type`, and `author` fields quoted in some support answers do not exist on the App entity: because unknown fields are silently dropped, requesting them yields no error and no value, so modelling them would have meant an `App.Enabled` that reads `false` for every app forever. Whether an app is on or off is a per-project property of its `AppUsage`, not of the `App`.

BUG FIXES:

## 1.2.0
FEATURES:
- `Scope` now carries a `Project` reference, so `AssignedRole` (`CreateAssignedRole`, `UpdateAssignedRole`, `GetAllAssignedRoles`, `GetAssignedRoleById`) can assign or read roles scoped to a specific project (`ProjectScope`) in addition to global scope, letting the Tofu/Terraform provider and other tools manage permissions at project or global level.
- Add `GetAssignedRolesByHolder(ctx, holderID, top, skip)` to look up all role assignments for a specific user or group, using the `holder` query filter on `api/assignedRoles`.
- `GetAllAssignedRoles` now takes `top, skip int` pagination parameters, matching the convention used by `ListUsers`/`ListServices`; pass 0 for both to keep the previous default-page behavior.

IMPROVEMENTS:
- `CreateAssignedRole` and `UpdateAssignedRole` now set `$type` to `AssignedRole` on the request payload automatically.
- `DeleteGroup`, `RemoveUserFromGroup`, `DeleteYoutrackRole`, `DeleteCustomField`, and `DeleteIssueLinkType` now reuse the existing shared helpers (`isRetryableMembershipEndpointError`, `sendMembershipRequest`, `deleteByID`) instead of each duplicating the same request/retry/not-found logic inline.

BUG FIXES:
- `GetAllAssignedRoles` previously unmarshaled the `api/assignedRoles` response as a `{"assignedRoles": [...]}` wrapper, but the YouTrack REST API returns a bare JSON array for this endpoint; it always returned an empty list. Fixed to parse the bare array.
- `GetUserByLogin`, `GetUserGroupByName`, and `GetAllUsersGroup` previously only checked the server's first page of results, so lookups could fail on instances with more users/groups than fit on one page — including `DeleteUser`'s internal lookup of the `guest` successor. All three now page through results until a match is found.
- `GetUserByLogin` now matches logins case-insensitively, consistent with `GetUserGroupByName`.
- `GetEnumBundleByName`/`GetStateBundleByName` could return a case-insensitive match found on an earlier page even when an exact match existed on a later page; the underlying paginated lookup now always prefers an exact match across all pages before falling back to a case-insensitive one.
- `UpdateService` and `UpdateYoutrackRole` now wait for the asynchronous update to settle before re-fetching, matching the fix already applied to `UpdateOAuth2AuthModule` in 1.1.6; previously they could return stale pre-update values.
- `DeleteProject` and `RemoveProjectCustomField` now treat a 404 response as success, making them idempotent like the other `Delete`/`Remove` methods in the client.
- `assignedRoleFields` now requests `role.key`, `role.permissions`, `holder.ringId`, and `holder.description`, which were previously omitted from the fields query and so always came back as zero values.
- `UpdateAppearanceSettings` now only sends the `dateFieldFormat`/`timeZone` field that is actually set, so updating one no longer resets the other to empty.

BREAKING CHANGES:
- Renamed the exported `AssignedRoles` type to `AssignedRole` (it represents a single role assignment) and removed `AssignedRolesResponse`, which was unused now that the list endpoint is parsed as a bare array. Consumers referencing either name need to update to `AssignedRole`.
- `GetAllAssignedRoles` gained required `top, skip int` parameters (see FEATURES above).

## 1.1.7
FEATURES:
- Add `CreateService`, `ListServices`, `GetServiceByID`, `UpdateService`, and `DeleteService` for managing Hub services (external application registrations, e.g. for OAuth-based integrations), covering the HUB-REST-API Services endpoints.
- `Service` now exposes the five OAuth 2.0 grant flow flags Hub tracks per service: `ClientCredentialsFlowEnabled`, `AuthCodeFlowEnabled`, `PKCERequired`, `ImplicitFlowEnabled`, and `ResourceOwnerFlowEnabled`.

IMPROVEMENTS:

BUG FIXES:

## 1.1.6
FEATURES:
- Add `CreateAzureAuthModule`, `GetAzureAuthModuleByID`, `UpdateAzureAuthModule`, and `DeleteAzureAuthModule` for managing Hub Microsoft Entra ID (formerly Azure AD) authentication modules, alongside the existing generic OAuth2 support.

IMPROVEMENTS:

BUG FIXES:
- `UpdateOAuth2AuthModule` now waits for Hub's asynchronous update to settle before re-fetching the module, so the returned state reflects the just-applied change instead of occasionally racing the update and returning stale values.
- `OAuth2AuthModule`'s optional string fields (`redirectUri`, `iconUrl`, `extensionGrantType`, `scope`, `userInfoUrl`, `idpLogoutUrl`, and the `user*Path`/`user*Url` claim-mapping fields) and optional int fields (`connectionTimeout`, `readTimeout`) no longer use `omitempty` on marshal, so clearing one of them to an empty/zero value on update now actually sends the clear to Hub instead of silently omitting the field and leaving the previous value in place. `syncInterval` keeps `omitempty`: Hub rejects the request outright if it's present but empty.

## 1.1.5
FEATURES:
- Add `BanUser` method to ban a user account via Hub lifecycle semantics.

IMPROVEMENTS:
- Expand `AddUserToGroup` fallback endpoint chain: canonical Hub usergroup membership endpoints (`POST /api/usergroups/{id}/users`) are now tried first, followed by user-centric endpoints (`PUT /api/users/{id}/groups/{id}`), improving compatibility across YouTrack and Hub versions.
- Include `banned` field in Hub user lifecycle queries so ban status is reflected when fetching or updating users.
- Include `description` field when listing groups.
- Add `Description` field to `Holder` model.

BUG FIXES:

## 1.1.4
FEATURES:

IMPROVEMENTS:
- Update dependencies

BUG FIXES:
- Add hub endpoint for group deletion to support YouTrack 2024.1+ where the legacy endpoint is no longer available. The new endpoint requires a successor group ID when deleting a group, which is now supported in the client.
- Harden group management using ringId for group identification to avoid issues with groups that have the same name. The client now uses ringId for group operations when available, falling back to name-based identification only when necessary.

## 1.1.3
FEATURES:

IMPROVEMENTS:
- Support for permission graph in role management, allowing to resolve implied and dependent permissions when creating or updating roles.

BUG FIXES:

## 1.1.2
FEATURES:

IMPROVEMENTS:
- Add default values bundles customfields

BUG FIXES:

## 1.1.1
FEATURES:

IMPROVEMENTS:
- Add default values for project customfield bundles

BUG FIXES:

## 1.1.0
FEATURES:
- Create/update/delete users
- Create/update/delete groups
- Add/remove users from groups
- List users and groups with pagination

IMPROVEMENTS:
- Add integration tests for user and group management.
- Update dependencies.

BUG FIXES:

## 1.0.2
FEATURES:

IMPROVEMENTS:
- Align release process with standard GitHub Actions workflow for tag-based releases.

BUG FIXES:
- Change Licence from GPL-3.0 to MPL-2.0 License.

## 1.0.1
FEATURES:

IMPROVEMENTS:

BUG FIXES:
- Rename organization to ELCAIT in package name and documentation.

## 1.0.0
FEATURES:
Initial release of the YouTrack API client library.

IMPROVEMENTS:
Extract code from youtrack provider so that it can be used as a standalone library for other use cases.

BUG FIXES:
