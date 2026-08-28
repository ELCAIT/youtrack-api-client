#!/usr/bin/env python3
"""Compare this client's Go structs against YouTrack's OpenAPI schema.

Reports fields the spec declares that the Go struct omits, and fields the Go
struct declares that the spec does not know about. Both directions matter: the
first is coverage the client is missing, the second is usually a field that
YouTrack silently ignores, which fails invisibly because YouTrack drops unknown
names from a `fields` query instead of rejecting them.

Findings are leads to verify against a live instance, never conclusions. The
spec is generated from JetBrains' server annotations and is known to disagree
with the running server in both directions -- see the skill for the confirmed
examples.

Usage:
    python3 drift.py --spec openapi.json [--client ./client] [--pair Go=Schema]
"""

import argparse
import json
import pathlib
import re
import sys

# Go struct name -> OpenAPI schema name, where the two differ or where the
# pairing is not simply identical. Identical names are matched automatically.
DEFAULT_PAIRS = {
    "Project": "Project",
    "Role": "Role",
    "Permission": "Permission",
    "ProjectCustomField": "ProjectCustomField",
    "ProjectTimeTrackingSettings": "ProjectTimeTrackingSettings",
    "WorkItemType": "WorkItemType",
    "WorkTimeSettings": "WorkTimeSettings",
    "BackupSettings": "BackupSettings",
    "AppearanceSettings": "AppearanceSettings",
    "LocaleSettings": "LocaleSettings",
    "RestSettings": "RestSettings",
    "GlobalSettings": "GlobalSettings",
    "License": "License",
    "IssueLinkType": "IssueLinkType",
    "CustomField": "CustomField",
    "Agile": "Agile",
    "Sprint": "Sprint",
    # Hub schema names are lowercase and differ from the YouTrack spelling.
    # These pair only against the Hub spec; they are ignored against the
    # YouTrack one, where the schemas do not exist.
    "Service": "service",
    "OAuth2AuthModule": "oauth2authmodule",
    "AzureAuthModule": "azureauthmodule",
}

# Go fields the spec does not declare but which are confirmed correct against a
# live instance. Suppressed from the "in Go, not in spec" report so that real
# drift stays visible. Each entry records how it was confirmed.
CONFIRMED_GO_FIELDS = {
    ("User", "password"): "write-only; absent from the read schema by design",
    # Hub spec says isDefault; the server returns and accepts `default`.
    ("OAuth2AuthModule", "default"): "server returns `default`, not the spec's `isDefault`",
    ("AzureAuthModule", "default"): "server returns `default`, not the spec's `isDefault`",
    ("EnumBundle", "name"): "returned live on 2026.2; spec omits it",
    ("StateBundle", "name"): "returned live on 2026.2; spec omits it",
    ("NestedGroup", "description"): "returned live on 2026.2; spec omits it",
    ("FieldType", "presentation"): "returned live on 2026.2; spec omits it",
    ("Permission", "key"): "returned live on 2026.2 and equals id; spec omits it",
}

# Fields intentionally absent from the Go models, with the reason. These are
# suppressed from the "missing" report so that real drift stays visible.
INTENTIONAL_OMISSIONS = {
    # Nested expansions the client fetches through dedicated methods instead.
    ("Project", "customFields"): "fetched via GetProjectCustomFields",
    ("Project", "issues"): "out of scope: issues are not covered",
    ("GlobalSettings", "appearanceSettings"): "fetched via GetAppearanceSettings",
    ("GlobalSettings", "localeSettings"): "fetched via GetLocaleSettings",
    ("GlobalSettings", "restSettings"): "fetched via GetRestSettings",
    ("GlobalSettings", "systemSettings"): "fetched via GetSystemSettings",
    ("GlobalSettings", "notificationSettings"): "not covered",
    ("ProjectTimeTrackingSettings", "workItemTypes"): "fetched via ListWorkItemTypes",
    ("ProjectTimeTrackingSettings", "project"): "back-reference, redundant",
    ("ProjectCustomField", "project"): "back-reference, redundant",
    ("CustomField", "instances"): "back-reference to project attachments",
    # Confirmed broken or empty on YouTrack 2026.2.
    ("Project", "startingNumber"): "HTTP 500 on 2026.2, see skill",
    # Hub: SSO/provisioning surface the client deliberately does not model.
    ("OAuth2AuthModule", "autoJoinGroups"): "SSO group mapping, not modelled",
    ("OAuth2AuthModule", "groupMappings"): "SSO group mapping, not modelled",
    ("OAuth2AuthModule", "attributeMappings"): "SSO claim mapping, not modelled",
    ("AzureAuthModule", "autoJoinGroups"): "SSO group mapping, not modelled",
    ("AzureAuthModule", "groupMappings"): "SSO group mapping, not modelled",
    ("AzureAuthModule", "attributeMappings"): "SSO claim mapping, not modelled",
}


def resolve_properties(schemas, name, seen=None):
    """Return a schema's properties, following allOf inheritance."""
    seen = seen or set()
    if name in seen or name not in schemas:
        return {}
    seen.add(name)

    schema = schemas[name]
    out = {}
    for sub in schema.get("allOf", []):
        if "$ref" in sub:
            out.update(resolve_properties(schemas, sub["$ref"].split("/")[-1], seen))
        out.update(sub.get("properties", {}))
    out.update(schema.get("properties", {}))

    return out


def subtype_properties(schemas, base):
    """Union the properties of every schema whose name ends with `base`.

    YouTrack models polymorphic entities as a bare base schema plus subtypes
    that carry the real fields (ProjectCustomField vs EnumProjectCustomField).
    A flattened Go struct legitimately holds the union, so subtype fields must
    not be reported as unknown.
    """
    out = {}
    for name in schemas:
        if name != base and name.endswith(base):
            out.update(resolve_properties(schemas, name))

    return out


def parse_go_structs(client_dir):
    """Map Go struct name -> {json tag: Go field name} for non-test files."""
    structs = {}
    for path in sorted(pathlib.Path(client_dir).glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        src = path.read_text()
        for match in re.finditer(r"type (\w+) struct \{(.*?)\n\}", src, re.S):
            name, body = match.group(1), match.group(2)
            tags = {}
            for field, tag in re.findall(
                r"(\w+)\s+[\w\.\*\[\]]+\s+`json:\"([^\",]+)", body
            ):
                tags[tag] = field
            structs[name] = tags

    return structs


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--spec", required=True, help="path to openapi.json")
    parser.add_argument("--client", default="client", help="path to the client package")
    parser.add_argument(
        "--pair",
        action="append",
        default=[],
        metavar="Go=Schema",
        help="additional struct/schema pairing; repeatable",
    )
    parser.add_argument(
        "--all",
        action="store_true",
        help="also report intentional omissions instead of suppressing them",
    )
    args = parser.parse_args()

    spec = json.loads(pathlib.Path(args.spec).read_text())
    schemas = spec.get("components", {}).get("schemas", {})
    version = spec.get("info", {}).get("version", "unknown")

    go_structs = parse_go_structs(args.client)

    pairs = dict(DEFAULT_PAIRS)
    for extra in args.pair:
        go, _, schema = extra.partition("=")
        pairs[go] = schema or go
    # Any struct whose name matches a schema exactly is worth checking too.
    for name in go_structs:
        if name in schemas:
            pairs.setdefault(name, name)

    print(f"YouTrack OpenAPI drift report (spec version {version})")
    print(f"schemas: {len(schemas)}   go structs: {len(go_structs)}")
    print()

    findings = 0
    for go_name in sorted(pairs):
        schema_name = pairs[go_name]
        if go_name not in go_structs or schema_name not in schemas:
            continue

        have = set(go_structs[go_name])
        want = set(resolve_properties(schemas, schema_name))
        # Subtype fields are legitimate on a flattened Go struct.
        known = want | set(subtype_properties(schemas, schema_name))

        missing, suppressed = [], []
        for field in sorted(want - have - {"$type"}):
            reason = INTENTIONAL_OMISSIONS.get((go_name, field))
            if reason and not args.all:
                suppressed.append(f"{field} ({reason})")
            else:
                missing.append(field)

        unknown, confirmed = [], []
        for field in sorted(have - known - {"$type"}):
            reason = CONFIRMED_GO_FIELDS.get((go_name, field))
            if reason and not args.all:
                confirmed.append(f"{field} ({reason})")
            else:
                unknown.append(field)

        if not (missing or unknown):
            continue

        findings += 1
        print(f"### {go_name}  (schema: {schema_name})")
        if missing:
            print(f"  in spec, not in Go ({len(missing)}):")
            print(f"    {', '.join(missing)}")
        if unknown:
            print(f"  in Go, not in spec ({len(unknown)}):")
            print(f"    {', '.join(unknown)}")
            print("    ^ verify on a live instance: YouTrack silently drops unknown")
            print("      field names, so these fail invisibly if the server ignores them.")
        if suppressed:
            print(f"  suppressed as intentional: {'; '.join(suppressed)}")
        if confirmed:
            print(f"  suppressed as confirmed-live: {'; '.join(confirmed)}")
        print()

    if findings == 0:
        print("No drift found in the paired structs.")

    print("Every finding is a lead, not a conclusion. Confirm each one against a")
    print("live instance before changing code -- the spec and the server disagree")
    print("in both directions.")

    return 0


if __name__ == "__main__":
    sys.exit(main())
