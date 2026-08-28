---
name: youtrack-openapi-drift
description: Use when checking this client's Go models against YouTrack's own OpenAPI specification — after a YouTrack version bump, before adding a resource, or when a field "should be there but isn't". Covers pulling /api/openapi.json from a live instance, running the drift script, and (critically) how to tell a real finding from the spec's many false positives, since the spec and the running server disagree in both directions. Pair with youtrack-api-integration when the outcome is adding an endpoint.
---

# Checking this client against YouTrack's OpenAPI spec

YouTrack publishes a generated OpenAPI 3.0 document at `/api/openapi.json` on
every instance. It is a genuinely useful **drift check** — it has already
caught a silently-ignored field and a missing operator-critical one — but it is
**not a source of truth**, and it must never be used to generate this client.
See "Why not generate" at the bottom before proposing that.

## Pulling the spec

```bash
curl -s -H "Authorization: Bearer $YOUTRACK_TOKEN" \
  "$YOUTRACK_BASE_URL/api/openapi.json" -o /tmp/openapi.json
```

`.envrc` normally provides both variables (see README.md "Integration Tests").
On YouTrack 2026.2 the document is ~560KB: 157 paths, 281 operations, 232
schemas. If it 404s and the response mentions `InstallerServletDispatcher`, the
instance has not finished its setup wizard — no API route exists yet.

## Running the drift check

```bash
python3 .claude/skills/youtrack-openapi-drift/scripts/drift.py \
  --spec /tmp/openapi.json --client client
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

- YouTrack's spec is a known-bad generator input: it produces
  [uncompilable Python](https://github.com/OpenAPITools/openapi-generator/issues/17910)
  and [uncompilable Kotlin](https://github.com/OpenAPITools/openapi-generator/issues/11306)
  via circular `allOf` inheritance and redefined inherited members.
- Generation cannot express what this client's models encode: the
  `omitempty`-versus-explicit-empty decisions in `Service` and
  `OAuth2AuthModule` exist because Hub *leaves omitted keys untouched*, so
  dropping a key keeps the old value instead of clearing it. A generator emits
  the naive struct and silently reintroduces those bugs.
- The apps/`usages` surface is undocumented and absent from the spec entirely.
- The operator-facing layer — the error taxonomy, the async read-back polling,
  the `ListAll*` walkers — is hand-written regardless.

Use the spec as a reference and a drift check. Keep the client hand-written.
