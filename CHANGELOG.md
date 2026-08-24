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
