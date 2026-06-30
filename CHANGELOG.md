## 1.1.4
FEATURES:

IMPROVEMENTS:

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
