# zenfra_stack_dependency: the provider resource for stack dependencies (zenfra-cloud#1001)

Revision 2, 2026-10-26. Approach agreed in a design round before writing:
id = the API edge id with a `child_stack_id/edge_id` import, a parent the
token cannot read is kept from state on refresh and refuses an import, a new
edge cannot be created disabled, refusal codes reach the diagnostic through a
client fix, `activated_at` is a plain computed attribute. Review round 1:
ship, with two notes taken in: `ModifyPlan` returns on a destroy plan first,
and the acceptance test removes the edge alone to prove DELETE. Round 2:
ship.

## Overview

Part of epic ZenfraCloud/zenfra-cloud#996 (stack dependencies, Release A, every
plan). Its decisions bind this plan: an edge is never edited; enable, disable
and delete are the only changes, so a different parent or child is a new edge.

The API is zenfra-cloud#998 (merged as zenfra-cloud d772ca4e, live). This
issue gives a customer who manages stacks as code a resource to declare
"after stack P applies, start a run of stack C" beside the two stacks:

```hcl
resource "zenfra_stack_dependency" "app_after_network" {
  parent_stack_id = zenfra_stack.network.id
  child_stack_id  = zenfra_stack.app.id
}
```

Out of scope: dispatch (#999), the dashboard and the zenfra-docs Dependencies
page (#1000), `last_dispatch`, `revision`, `created_at`, `created_by` (not
asked for; they can be added as computed attributes when someone needs them),
a data source.

## Context (from discovery)

API, read in `../zenfra-cloud` at d772ca4e:

- Handler `zenfra-api/internal/handler/stack_dependency.go`: request and
  response structs at 41-75. Routes, all authorized on the path stack, which
  is the edge's **child** for every write:
  - `POST /api/v1/stacks/{child}/dependencies` `{"parent_stack_id"}` → 201 one edge
  - `GET /api/v1/stacks/{id}/dependencies` → 200 `{"upstream": [...], "downstream": [...]}`
  - `PUT /api/v1/stacks/{child}/dependencies/{edge}/enabled` `{"enabled": bool}` → 200 one edge
  - `DELETE /api/v1/stacks/{child}/dependencies/{edge}` → 204
- There is no single-edge GET. An edge is
  `{id, stack: {id, name} | null, enabled, activated_at, revision, created_at, created_by, last_dispatch}`;
  `stack` is the other end as seen from the path stack (the parent, in the
  child's `upstream` list and in the POST and PUT responses), null when the
  caller cannot read it or it is not live. No raw parent or child id is ever
  returned (998 plan decision 10).
- POST always creates an **enabled** edge
  (`service/stack_dependency.go:181-184`); cycle and depth are checked on the
  enabled edges plus the new one (`:171-175`).
- Refusals are 409 `{"error": <code>, "message": ...}` (`handler/stack.go:1101-1108`):
  `dependency_self`, `dependency_exists`, `dependency_cycle`,
  `dependency_depth_reached`, `dependency_fanout_reached`, in that precedence
  (998 decision 4). At enable `dependency_exists` does not apply.
- A same-state PUT is 200 with the stored edge and changes nothing
  (998 decision 6; `service/stack_dependency.go:227-228`), so PUT is safe to
  retry. `activated_at` is set at creation and at every enable that changes
  state, as RFC3339 with milliseconds (998 decision 7; handler fixture
  `"2026-10-25T12:00:00.123Z"`, `handler/stack_dependency_test.go:129`).
- A stack that is the parent of any live edge cannot be deleted
  (`stack_has_dependents`, 998 decision 9); deleting a child soft-deletes its
  incoming edges. So `stack: null` on a child's upstream edge means, in
  practice, that the token cannot read the parent.
- Create and enable need `stack.triggers_write` on the child and `stack.read`
  on the parent; disable and delete need only `stack.triggers_write` on the
  child (998 decision 5), so a disable response can carry `stack: null`.

This repository:

- Layout to copy: `internal/resource/webhook_endpoint/` (resource, model,
  `_resource_test.go` against an httptest API, `_acc_test.go`); composite
  import parsing in `internal/resource/space_grant/space_grant_resource.go:163-172`.
- Client: `internal/zenfraclient/client.go:186-232` `checkResponse` fills
  `APIError.Code` only from a `"code"` member and uses `"error"` as the
  message fallback, so a dependency refusal arrives today as a
  `ConflictError` with `Code == ""`. `errors.go:15-16` documents `Code` as
  the `"code"` member only. Client methods wrap errors (`fmt.Errorf("...: %w")`),
  so callers use `errors.As`.
- `CreateWebhookEndpoint` (`internal/zenfraclient/webhook_endpoints.go:15-27`)
  sends exactly one POST by copying the client with `retry.maxRetries = 0`.
- Acceptance harness `internal/acctest/acctest.go`: `PreCheck`, `Client()`,
  `SpaceID()`, `StackHCL(resourceName, name, extra)` on the public fixture
  repository; needs `TF_ACC=1`, `ZENFRA_API_ENDPOINT`, `ZENFRA_API_TOKEN`,
  `ZENFRA_ACC_SPACE_ID`.
- Registry docs: `make docs` builds the provider, dumps the schema and runs
  `tfplugindocs` (`tools/tools.go`); `templates/` holds only `index.md.tmpl`,
  so a resource page comes from the schema descriptions plus
  `examples/resources/<type>/resource.tf` and `import.sh`. Needs `terraform`
  and `python3` on the path.
- Registration: `internal/provider/provider.go:131-146`. Resource tables in
  `CLAUDE.md` ("Resources (13)") and `README.md:31`.
- This repository has no `docs/plans/completed/` yet; the implementation
  creates it.

## Settled decisions

These are not re-opened during implementation.

1. **`id` is the API edge id; import takes `<child_stack_id>/<edge_id>`.**
   There is no single-edge GET and the edge carries no raw child id, so the
   child must come with the import id. Split on `/` exactly as space_grant
   does, both parts non-empty, else "Invalid Import ID" naming the expected
   format. This deviates from the issue's "import by id" on purpose; the
   reason goes in the `ImportState` comment and the schema description.
   Ruled out: a composite `id` with passthrough import (the attribute would no
   longer be the API's edge id, which the rest of the epic's tooling names);
   import by bare edge id (would scan every stack's dependencies, and fails on
   unreadable stacks anyway).
2. **Read lists the child's dependencies and finds the edge by id in
   `upstream` only.** Child 404, or no upstream edge with that id → the edge
   is gone, `RemoveResource`. Any other error → diagnostic.
3. **`stack: null` keeps the known parent.** On refresh and in the PUT
   response, `parent_stack_id` is taken from `stack.id` when present and kept
   from prior state when null (the same move the webhook resource makes for
   its secret). An edge is never edited, so the kept value cannot be stale.
   On import there is no prior value: a null `stack` fails the import with a
   diagnostic that the token cannot read the parent stack of that edge.
   Null is never treated as drift: a replacement cannot be planned without a
   value.
4. **A new edge cannot be created disabled.** The API creates enabled edges
   only. Create-then-disable was ruled out: the edge is live and enabled until
   the PUT lands, which is not bounded to one round trip when the PUT retries,
   times out or fails, and work dispatch has admitted stays admitted after a
   disable (epic #996, Admission). It would also be checked for cycle and
   depth as an enabled edge first. So:
   - the resource implements `ModifyPlan`. It returns at once when
     `req.Plan.Raw.IsNull()` (a destroy, which must never be refused, also
     for a disabled edge), as `stack_variables_resource.go:99-101` does; it
     does **not** copy their `State.Raw.IsNull()` return, because a create is
     exactly the case it checks. Then, when the plan creates the edge
     (prior state null) or replaces it (planned `parent_stack_id` or
     `child_stack_id` not equal to state, which includes an unknown value) and
     `enabled` is known `false`, it adds an error diagnostic at
     `path.Root("enabled")`: a new dependency is always created enabled;
     create it with `enabled = true` (or omit it) and disable it in a later
     apply;
   - `Create` repeats the check before the POST, for a value only known at
     apply;
   - no schema validator: it would also reject a valid update to `false` and
     an imported disabled edge;
   - the `enabled` description states the limitation.
   Atomic disabled creation needs an API change and is outside this issue.
5. **`activated_at` is plain `Computed`, with no plan modifier.** It changes
   on every enable that changes state; `UseStateForUnknown` would plan it as
   unchanged and fail with "inconsistent result after apply". Since `enabled`
   is the only in-place change, it is unknown exactly when it may change. It
   is kept as the string the API sends (DTO field `string`), never parsed and
   reformatted: `time.Time` formatting does not always reproduce three
   fractional digits.
6. **Create sends exactly one POST** (the `CreateWebhookEndpoint` pattern): a
   replayed create the API had committed answers `dependency_exists` with
   nothing in state. List, PUT and DELETE keep the client's normal retries.
7. **Refusal codes reach the diagnostic.** `checkResponse` keeps an explicit
   `"code"` member first; when it is absent and `"message"` is non-empty, the
   `"error"` member is the code. A body with only `"error"` stays a message,
   as today. The `errors.go` comment on `Code` says so. The resource, on an
   error that `errors.As` into `*zenfraclient.ConflictError` with a non-empty
   `Code`, at create and at enable, reports summary `Stack Dependency Refused`
   and detail `<code>: <message>`; every other error keeps the generic
   `Error Creating/Updating Stack Dependency` form.
8. **Schema**: `id` (computed, `UseStateForUnknown`), `parent_stack_id` and
   `child_stack_id` (required, `RequiresReplace`), `enabled` (optional +
   computed, default `true`), `activated_at` (computed, decision 5).
9. **Update** handles `enabled` only: PUT when plan and state differ, else no
   call; the new state comes from the PUT response (with decision 3 for
   `stack`). A refusal at enable leaves state unchanged (`enabled = false`),
   so the next plan offers the enable again.
10. **Delete** calls DELETE; a 404 (edge or child already gone) is success.
11. **The cycle refusal is an apply-time diagnostic.** The issue says "plan
    error"; the API decides at create and enable, so the provider learns it
    at apply. The acceptance test expects the error on that apply step. The
    only plan-time error is decision 4's.
12. **Documentation.** Registry docs are generated in this PR. The one-line
    mention on the zenfra-docs Dependencies page belongs to zenfra-cloud#1000,
    so this PR has no zenfra-docs worktree and its `Customer docs` line names
    the registry page and says so.

## Development Approach

- **testing approach**: regular (code, then its tests in the same task)
- complete each task fully before moving to the next; tests in every task
  cover success and failure; `make test` green before the next task
- every new file starts with the two `// ABOUTME:` lines
- update this plan when scope changes during implementation

## Testing Strategy

- **unit**: client DTO and error parsing against the handler's JSON;
  resource CRUD, import, plan modification and diagnostics against an
  httptest fake API, the webhook resource tests' pattern
- **acceptance**: `internal/resource/stack_dependency/stack_dependency_acc_test.go`,
  three stacks from `zacc.StackHCL`; runs only with `TF_ACC=1` and the three
  variables
- no e2e suite in this repository

## Implementation Steps

### Task 1: Client: refusal code from the `error` member

**Files:**
- Modify: `internal/zenfraclient/client.go`
- Modify: `internal/zenfraclient/errors.go`
- Modify: `internal/zenfraclient/client_test.go`

- [x] in `checkResponse`, after reading the body: `Code` from `"code"`; when
      empty and `"message"` is non-empty, `Code` from `"error"` (decision 7)
- [x] update the `APIError.Code` comment in `errors.go`
- [x] test: `{"error":"dependency_cycle","message":"..."}` on 409 →
      `ConflictError` with that code and message
- [x] test: explicit `"code"` wins over `"error"`; `{"error":"some prose"}`
      alone stays the message with an empty code; `IsAutoAttached` unchanged
- [x] `make test` green

### Task 2: Client: stack dependency DTOs and methods

**Files:**
- Modify: `internal/zenfraclient/types.go`
- Create: `internal/zenfraclient/stack_dependencies.go`
- Modify: `internal/zenfraclient/types_test.go`
- Modify: `internal/zenfraclient/client_test.go`

- [x] `types.go`: `CreateStackDependencyRequest{ParentStackID}`,
      `SetStackDependencyEnabledRequest{Enabled bool}`,
      `DependencyStackRef{ID, Name}`, `StackDependency{ID, Stack *DependencyStackRef,
      Enabled, ActivatedAt string, Revision int64, CreatedAt string, CreatedBy string}`,
      `StackDependencyList{Upstream, Downstream []StackDependency}`, mirroring
      handler lines 41-75 (`last_dispatch` not decoded: always null, not exposed)
- [x] `stack_dependencies.go`: `CreateStackDependency(ctx, childID, parentID)`
      (one POST, decision 6), `ListStackDependencies(ctx, stackID)`,
      `SetStackDependencyEnabled(ctx, childID, edgeID, enabled)`,
      `DeleteStackDependency(ctx, childID, edgeID)`; path segments escaped
      with `url.PathEscape`, errors wrapped as the other files do
- [x] test (`types_test.go`): decode the handler fixture shape: a list with an
      upstream edge whose `stack` is set and a downstream edge whose `stack`
      is null, `activated_at` `"2026-10-25T12:00:00.123Z"` kept verbatim,
      `last_dispatch: null`; marshal both requests to the exact JSON
- [x] test (httptest): each method's method, path and body; create is not
      retried on 503; a 409 refusal surfaces as a `ConflictError` with its code
- [x] `make test` green

### Task 3: Resource: schema, model, CRUD and import

**Files:**
- Create: `internal/resource/stack_dependency/stack_dependency_resource.go`
- Create: `internal/resource/stack_dependency/stack_dependency_model.go`
- Create: `internal/resource/stack_dependency/stack_dependency_resource_test.go`
- Modify: `internal/provider/provider.go`

- [x] model and `toState(edge, priorParent, childID)` implementing decision 3
- [x] schema per decision 8, with descriptions that state: the edge starts a
      run of the child after each successful tracked apply of the parent; a
      changed parent or child replaces the edge; `enabled` cannot be `false`
      on a new edge (decision 4); the import format and why (decision 1)
- [x] Create (decision 4 guard, one POST, refusal diagnostic), Read
      (decision 2), Update (decision 9), Delete (decision 10), ImportState
      (decision 1, then the read with the null-parent refusal of decision 3)
- [x] `ModifyPlan` per decision 4
- [x] register `NewStackDependencyResource` in `provider.go`
- [x] tests, success: create maps id, parent, child, enabled, activated_at;
      read finds the edge in `upstream` and ignores a `downstream` edge with
      the same id; disable and re-enable call PUT and take `activated_at`
      from the response; no call when nothing changed; import of
      `child/edge` fills every attribute
- [x] tests, failure and edge cases: child 404 and absent edge remove the
      resource; delete treats 404 as success; `stack: null` on read and on a
      disable response keeps the prior parent; `stack: null` on import is an
      error; malformed import ids (`x`, `/e`, `c/`, `a/b/c`); each refusal
      code at create and at enable gives `Stack Dependency Refused` with
      `<code>: <message>`, and an enable refusal leaves state unchanged;
      `ModifyPlan` errors on create and on replacement with known
      `enabled = false`, passes an update to `false`, a create with `true`,
      and an unknown `enabled`; a destroy plan (null plan) of a disabled edge
      passes without error; Create refuses `false` before any request;
      `parent_stack_id` and `child_stack_id` both carry `RequiresReplace`
- [x] `make test` and `make lint` green

### Task 4: Acceptance test

**Files:**
- Create: `internal/resource/stack_dependency/stack_dependency_acc_test.go`

- [x] config helper: three stacks `a`, `b`, `c` from `zacc.StackHCL` and the
      dependency resource(s) under test
- [x] steps, in order:
      1. `a → b` created; `enabled = true`, `activated_at` set, id recorded;
         the API's list for `b` holds it in `upstream`
      2. `enabled = false`: `ExpectResourceAction Update`, same id,
         `activated_at` unchanged
      3. `enabled = true`: Update, same id, `activated_at` changed
      4. import with `ImportStateIdFunc` building `<b id>/<edge id>`,
         `ImportStateVerify`
      5. parent `a` → `c`: `ExpectResourceAction Replace`, a different id
      6. add `b → c` beside the enabled `c → b`:
         `ExpectError: regexp.MustCompile("dependency_cycle")` (apply-time,
         decision 11)
      7. a new `zenfra_stack_dependency` with `enabled = false`:
         `ExpectError` on the plan-time message of decision 4
      8. back to `c → b` alone: `ExpectEmptyPlan`
      9. the three stacks without the edge: the edge is destroyed while `b`
         still exists, and a check through `zacc.Client()` finds its id
         absent from `b`'s `upstream`. This proves DELETE on its own: at the
         final destroy, deleting the child also removes its incoming edges
         (998 decision 8), which would hide a no-op DELETE
- [x] destroy runs edges before stacks (the edge references both stack ids);
      no `CheckDestroy` beyond the framework's
- [x] compiles and is skipped under `make test`; run under `make testacc`
      when the three variables are available, and say in the PR whether it ran

### Task 5: Verify acceptance criteria

- [x] every attribute and behaviour in the issue's "What it delivers" is
      covered, with the two recorded deviations (decision 1 import format,
      decision 11 apply-time cycle refusal) and decision 4's limitation
- [x] `make fmt`, `make lint`, `make test` green; `make testacc` result
      recorded (ran, or not run for want of credentials)
- [x] `make build` green

### Task 6: [Final] Documentation

No zenfra-docs page in this issue (decision 12): the Dependencies page line
is zenfra-cloud#1000's.

**Files:**
- Create: `examples/resources/zenfra_stack_dependency/resource.tf`
- Create: `examples/resources/zenfra_stack_dependency/import.sh`
- Create: `docs/resources/stack_dependency.md` (generated)
- Modify: `providers-schema.json` (generated)
- Modify: `CLAUDE.md`
- Modify: `README.md`
- Move: this plan to `docs/plans/completed/`

- [x] `resource.tf`: two stacks' ids and the edge, as in the Overview;
      `import.sh`: `terraform import zenfra_stack_dependency.app_after_network $CHILD_STACK_ID/$DEPENDENCY_ID`
- [x] `make docs`; review the generated page reads correctly (decision 4
      limitation and import format visible)
- [x] `CLAUDE.md`: resource list in the structure block, "Resources (14)",
      a table row (`parent_stack_id`/`child_stack_id` force replacement,
      `enabled` the only in-place change and never `false` on create, import
      `child_stack_id/edge_id`, refusal codes from the `error` member);
      `README.md` resource list line
- [x] `git mv` the plan to `docs/plans/completed/`

### Task 7: Pull request

- [ ] title without issue prefix noise, body per the writing-pr-descriptions
      skill; `Closes ZenfraCloud/zenfra-cloud#1001`
- [ ] `Customer docs:` the registry page `docs/resources/stack_dependency.md`
      ships in this PR; the zenfra-docs Dependencies line is
      ZenfraCloud/zenfra-cloud#1000's
- [ ] rollout: none needed, the API (zenfra-cloud#998) is live; the resource
      reaches customers with the next provider release
- [ ] the acceptance run result stated (decision in Task 4)

## Implementation notes

- `providers-schema.json` is gitignored (`.gitignore:45`); `make docs` writes
  it locally and nothing is committed for it.
- The acceptance test was not run: `ZENFRA_API_ENDPOINT` and
  `ZENFRA_ACC_SPACE_ID` were not available. It compiles and skips under
  `make test`.
- terraform-plugin-framework is v1.19.0 (go.mod), whose update pre-fills the
  response with the prior state, so a refused enable keeps state without an
  explicit restore.
- The resource inventory in `CLAUDE.md` and `README.md` also gained the
  missing `zenfra_cloud_integration_attachment` entry.

## Post-Completion

- After a provider release, zenfra-cloud#1000's docs change can link the
  registry page.
- An API option to create an edge disabled would lift decision 4's
  limitation; file it in zenfra-cloud only if a customer asks.
