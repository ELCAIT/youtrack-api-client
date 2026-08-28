---
name: youtrack-openapi-drift
description: Use when checking this client's Go models against the YouTrack and Hub OpenAPI specs — after a version bump, before adding or changing a resource's fields, or when a field "should be there but isn't". Establishes the rule that the spec is a draft and the live instance is the authority: covers pulling both specs (/api/openapi.json and /hub/api/rest/openapi.json), running the drift script, confirming each finding against a real server, and the catalogue of known false positives, since the specs and the server disagree in both directions. Pair with youtrack-api-integration when the outcome is adding an endpoint.
---

# Checking this client against the OpenAPI specs

This client wraps **two** APIs, and each publishes its own generated OpenAPI
3.0 document. Check both — they cover different halves of this client, and
neither is a superset of the other:

| Spec | Path | Covers |
| --- | --- | --- |
| YouTrack | `/api/openapi.json` | projects, custom fields, bundles, issue link types, global settings, apps |
| Hub | `/hub/api/rest/openapi.json` | users, groups, roles, permissions, auth modules, services |

Most of this client's identity and auth surface is **Hub**, so checking only
the YouTrack spec leaves `Service`, `OAuth2AuthModule`, and `AzureAuthModule`
unverified.

**The rule this skill exists to enforce: the spec is the draft, the live
instance is the authority.** Use a spec to discover what fields a resource
*might* have — it is far faster than reading HTML docs and it is how the two
missing `immutable` fields were found. Then confirm every field against a real
server before committing to it, because both specs disagree with the running
server in both directions, and the failure mode is silent: YouTrack drops
unknown field names from a query instead of rejecting them, so a wrong struct
produces a permanently zero value and no error.

Never generate the client from a spec — see "Why not generate" at the bottom
before proposing it.

## Pulling the specs

```bash
curl -s -H "Authorization: Bearer $YOUTRACK_TOKEN" \
  "$YOUTRACK_BASE_URL/api/openapi.json" -o /tmp/openapi.json

curl -s -H "Authorization: Bearer $YOUTRACK_TOKEN" \
  "$YOUTRACK_BASE_URL/hub/api/rest/openapi.json" -o /tmp/hub-openapi.json
```

`.envrc` normally provides both variables (see README.md "Integration Tests").
On 2026.2 the YouTrack document is ~560KB (157 paths, 281 operations, 232
schemas) and the Hub one ~440KB (169 paths, 276 operations, 189 schemas). If a
request 404s and the response mentions `InstallerServletDispatcher`, the
instance has not finished its setup wizard — no API route exists yet.

**Hub schema names are lowercase** (`service`, `oauth2authmodule`,
`azureauthmodule`) and so do not auto-pair with the Go struct names. The script
carries those pairings already; add `--pair Go=lowercasename` for new ones.

## Running the drift check

```bash
# Run it once per spec; findings differ between them.
python3 .claude/skills/youtrack-openapi-drift/scripts/drift.py \
  --spec /tmp/openapi.json --client client

python3 .claude/skills/youtrack-openapi-drift/scripts/drift.py \
  --spec /tmp/hub-openapi.json --client client
```

It pairs each Go struct with the schema of the same name (plus explicit
pairings for the ones that differ) and reports two directions:

- **in spec, not in Go** — coverage the client may be missing.
- **in Go, not in spec** — usually the more serious one. YouTrack **silently
  drops unknown names from a `fields` query instead of rejecting them**, so a
  field the server does not know about fails invisibly: no error, just a
  permanently zero value.

Flags: `--all` also prints the suppressed entries; `--pair Go=Schema` adds a
pairing the script cannot infer.

## Read every finding as a lead, never a conclusion

**This is the important part of this skill.** The spec is generated from server
annotations and disagrees with the running server in both directions. Confirm
each finding with `curl` against a live instance before touching code. Four
failure modes seen in practice, all confirmed on 2026.2:

1. **The spec omits fields the server really returns.** `EnumBundle.name`,
   `StateBundle.name`, `NestedGroup.description`, `FieldType.presentation`, and
   `Permission.key` are all absent from the schema and all returned live. The Go
   structs are right and the spec is wrong. These are suppressed in the script's
   `CONFIRMED_GO_FIELDS`.
   **The Hub spec also disagrees on a name:** it declares `isDefault` on the
   auth modules, but the server returns and accepts `default`, which is what the
   Go structs use. Renaming the tag to match the spec would silently break the
   field. Always confirm a rename against the server before making it.
2. **The spec declares fields the server cannot serve.** `Project.startingNumber`
   is in the schema and returns **HTTP 500** on a live instance
   (`Cannot invoke "java.lang.Number.longValue()" ... is null`). `createdBy` and
   `iconUrl` return `null`; `team` returns an empty object.
3. **Polymorphism looks like drift.** YouTrack models polymorphic entities as a
   near-empty base schema plus subtypes carrying the real fields —
   `ProjectCustomField` versus `EnumProjectCustomField`,
   `StateProjectCustomField`, and nine others. A flattened Go struct holding the
   union is *correct*; the script already unions subtype properties to avoid
   this false positive. Do not "fix" a flattened struct to match a bare base
   schema.
4. **Write-only fields are absent from read schemas by design.**
   `User.password` is the example. Not drift.

The reverse also happens, which is why the check is worth running: **a field can
be absent from the spec *and* genuinely dead.** `Role.key` is the confirmed
case — see below.

## How to confirm a finding

For a field the Go struct has but the spec does not, the decisive test is
whether the server round-trips it:

```bash
# 1. Read: ask for the field explicitly.
curl -s -H "Authorization: Bearer $YOUTRACK_TOKEN" \
  "$YOUTRACK_BASE_URL/api/roles/17-1?fields=id,name,key"

# 2. Control: ask for a field that certainly does not exist.
#    If both come back identical, the server is dropping unknown names and
#    absence tells you the field is not real.
curl -s -H "Authorization: Bearer $YOUTRACK_TOKEN" \
  "$YOUTRACK_BASE_URL/api/roles/17-1?fields=id,name,totallyBogusField"

# 3. Write: create with the field set, read it back, then clean up.
```

Always delete anything you create on the instance.

For a field the spec has but Go lacks, just request it and see whether a real
value comes back — a `null`, an empty object, or a 500 means it is not worth
modelling.

## Confirmed findings so far (YouTrack 2026.2)

- **`Role.key` does not exist.** Absent from the schema; never returned even
  when requested; silently ignored on create (verified by creating a role with
  `"key":"my-custom-key"` and reading back no key). It is marked `Deprecated:`
  in `roles_models.go` and removed from the `roleFields` list. It stays in the
  struct only because removing it would break the API; drop it in the next
  major version. **`Permission.Key` is unaffected** — that one is real,
  populated, and equal to `id`.
- **`Role.immutable` was missing and matters.** Built-in roles cannot be
  modified or deleted; on a stock instance 6 of 9 roles report `true`. Without
  it an operator retries doomed writes forever. Added, wired into `roleFields`,
  and verified live.
- **`Service.immutable` was missing, same story** (found via the *Hub* spec, not
  the YouTrack one). The bundled services — "YouTrack", "YouTrack
  Administration", "YouTrack Mobile", "Konnector" — all report `true`; on a
  stock instance 4 of 5 are immutable. Added, wired into `serviceFields`, and
  verified live.
- **`SavedQuery.pinned` is absent from the spec but returned live** (found
  while dry-running the workflow on a resource this client does not yet cover).
  A struct drafted from the schema alone would have missed it. Recorded here as
  a standing example of why step 3 of the `youtrack-api-integration` checklist
  is not optional.
- **Auth-module SSO surface is deliberately unmodelled.** `autoJoinGroups`,
  `groupMappings`, and `attributeMappings` are real Hub features but return
  absent on an instance that does not configure them. Suppressed as intentional;
  model them only when a caller actually needs SSO group/claim mapping.

## When to run this

- After a YouTrack version bump — the highest-value moment.
- Before adding a resource, to get its real field list (pair with
  `youtrack-api-integration`).
- When a field is unexpectedly zero: check whether the server knows the name.

## Adding a field you have confirmed

Adding it to the Go struct is **not enough**. Every request sends an explicit
`fields=` list, so a field missing from the resource's `xFields` constant will
never populate no matter how the struct is declared. This bit `Role.Immutable`
during its own addition. See `youtrack-go-conventions` for the struct and
const-block style, and record the finding in `CHANGELOG.md` with the evidence.

## Why not generate the client from this spec

Asked and answered; do not re-litigate without new information.

- **The generated models would be wrong in exactly the places that matter.**
  This was tested, not assumed: `openapi-generator -g go` on the 2026.2 spec
  emits 233 model files that **do compile** (Go's struct embedding absorbs the
  118-of-232 schemas that redefine an inherited property). The problem is
  fidelity, not compilation. The generated `EnumBundle` has **no `Name` field**,
  because the spec omits it — yet the server returns one. Unmarshal a real
  response into the generated type and the name is silently dropped; re-marshal
  and you write it back as absent. Every spec-omits-reality case in the false
  positives above becomes a silent data-loss bug in a generated client, and the
  drift check that would have caught it only works because there is a
  hand-written struct to compare against.
  Other languages fare worse — the same spec produces
  [uncompilable Python](https://github.com/OpenAPITools/openapi-generator/issues/17910)
  and [uncompilable Kotlin](https://github.com/OpenAPITools/openapi-generator/issues/11306)
  — but for Go the argument is fidelity and the points below, not compilation.
- Generation cannot express what this client's models encode: the
  `omitempty`-versus-explicit-empty decisions in `Service` and
  `OAuth2AuthModule` exist because Hub *leaves omitted keys untouched*, so
  dropping a key keeps the old value instead of clearing it. A generator emits
  the naive struct and silently reintroduces those bugs.
- The apps/`usages` surface is undocumented and absent from the spec entirely.
- The operator-facing layer — the error taxonomy, the async read-back polling,
  the `ListAll*` walkers — is hand-written regardless.

Use the spec as a reference and a drift check. Keep the client hand-written.
