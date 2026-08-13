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
