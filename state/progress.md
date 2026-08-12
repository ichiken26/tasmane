# Progress Log

## 2026-08-13 — Phase 2 graph and scoring defense-in-depth

- Relation use cases reject any loaded graph task whose owner differs from the authenticated user, preventing a faulty repository adapter from bypassing tenant isolation.
- Dependency validation now rejects cycles anywhere in the supplied active graph, including an existing cycle that is not reachable from the proposed edge.
- Confirmation tests fix all nine status transition pairs as an explicit adjacent-only FSM table.
- TaskScore tests fix the documented 30/25/20/15/5/5 composition with exact neutral and maximum-component results.
- Red reproduced foreign-graph persistence and remote-cycle acceptance; Green confirmed the shared Domain/Application fixes.
- Verification: Go format, vet, unit tests, race tests, and build; Nuxt 21 tests, lint, typecheck, and production build; ephemeral PostgreSQL integration; full `scripts/verify.sh`; `git diff --check` — all passed.

## 2026-08-13 — repair round 4 (scoring and Confirmation invariant hardening)

- Red: domain regression tests reproduced acceptance of zero scoring times, empty Task IDs, unknown Task status, and zero `confirmed_at`; boundary tests exercised platform maximum dependency counts.
- Green: Priority and TaskScore now validate required identity/status/time inputs before scoring, including completed Tasks, and reject zero explicit deadlines.
- Dependency contributions use saturating arithmetic, preventing integer overflow while preserving the configured `0..100` caps for maximum `int` inputs.
- Confirmation FSM now requires a non-zero timestamp for `CHECKING -> CONFIRMED` while transitions that clear `confirmed_at` continue to accept a zero timestamp.
- Validation: gofmt clean; `go vet ./...`, `go test ./...`, `go test -race ./...`, `go build ./...`, `npm test`, `npm run lint`, `npm run typecheck`, `npm run build`, fresh PostgreSQL integration, and `scripts/verify.sh` all passed with Go 1.22.12.

## 2026-08-13 — repair round 3 (shared mutation boundaries and invalid scoring input)

- Added shared application mutation boundaries for hierarchy assignments and dependency additions; all persistence calls occur only after the domain cycle, ownership, Genre, and existence rules pass.
- Added a shared Confirmation transition persistence boundary; prohibited FSM transitions cannot reach the repository. Existing Task application transitions continue to enforce unresolved-Confirmation and incomplete-descendant completion constraints for Web, MCP, and Google callers.
- Priority and TaskScore now reject negative dependency counts and invalid indirect distances (`<= 1`) with `ErrInvalidScoringInput`.
- ConfirmationScore now distinguishes confirmed exclusion from malformed input and rejects unknown status, parent score outside `0..100`, and zero evaluation time with `ErrInvalidConfirmationInput`.
- TDD coverage proves invalid writes do not reach relation/Confirmation repositories and covers numeric/status/time boundaries.
- Validation: gofmt clean; `go vet ./...`, `go test ./...`, `go test -race ./...`, `go build ./...`, `npm test`, `npm run lint`, `npm run typecheck`, `npm run build`, fresh PostgreSQL integration, and `scripts/verify.sh` all passed with Go 1.22.12.

## 2026-08-13 — Phase 2 graph, scoring, and confirmation domain

- Shared hierarchy assignment validation rejects self/descendant cycles, missing tasks, and cross-user/cross-Genre parents at arbitrary depth.
- Shared dependency validation rejects direct/indirect cycles, missing tasks, and cross-user/cross-Genre edges without coupling rules to a Transport.
- Priority calculation derives deadline/progress/dependency scores, supports nearest-descendant virtual deadlines, applies exact LOW/MEDIUM/HIGH boundaries, and honors validated manual priority overrides.
- TaskScore produces bounded deterministic rankings and reasons, dependency distance decay/caps, unmet-dependency and CHECKING penalties, overdue relaxation, completed exclusion, and deadline/ID tie breaks.
- Confirmation FSM enforces adjacent transitions and confirmed timestamps; ConfirmationScore combines due urgency, status, and parent TaskScore while excluding confirmed items.
- All scoring coefficients, thresholds, caps, and penalties live in `internal/domain/scoring/constants.go` for isolated tuning.
- Table and behavior tests cover graph cycles, invalid/missing/cross-user/cross-Genre relations, scoring boundaries, virtual deadlines, deterministic reasons/ties, overdue CHECKING behavior, FSM transitions, and ConfirmationScore inclusion/exclusion.
- Final review hardened malformed graph input handling for empty/duplicate task identities and invalid pre-existing dependency edges.
- Validation: gofmt clean; `go vet ./...`, `go test ./...`, `go test -race ./...`, `go build ./...`, `npm test`, `npm run lint`, `npm run typecheck`, `npm run build`, fresh PostgreSQL integration, and `scripts/verify.sh` all passed with Go 1.22.12.

## 2026-08-13 — repair round 5 (UUIDv7 database variant hardening)

- Additive `0003` migration upgrades every entity UUIDv7 constraint to require both version 7 and the RFC 9562 variant (`8`, `9`, `a`, or `b`).
- Fresh PostgreSQL integration coverage rejects a version-7-shaped UUID with a non-RFC variant and expects all three migrations to remain idempotent.
- Existing migrations remain unchanged.

## 2026-08-13 — repair round 4 (Phase 2 parent progress and rollback)

- Parent progress now derives a direct-child priority-weighted value with HIGH=5, MEDIUM=3, LOW=1, zero for not-started children, floor rounding, and the non-completed 95 cap.
- Domain rollback propagation reopens completed ancestors to IN_PROGRESS/95 when a child is reopened and resets only in-progress descendants when a parent returns to NOT_STARTED.
- Live PostgreSQL verification now launches eight independent OS processes updating one row and proves every trigger-generated `updated_at` value is unique.
- TDD coverage includes weighted progress, cap/completed behavior, invalid inputs, immutable rollback results, and completed-descendant preservation.

## 2026-08-12 — repair round 5 (Phase 1 database policies)

- PostgreSQL 14 packages extracted into a temporary directory without Docker or system installation; isolated loopback instance used for a live fresh-schema test.
- Existing `0001` left unchanged; additive `0002_lifecycle_policies.sql` enforces UUIDv7 entity IDs and installs automatic `updated_at` triggers.
- Live migration test applies the embedded migration set three times, verifies exactly-once history, case-insensitive uniqueness, UUIDv4 rejection, and serialized concurrent `updated_at` updates.
- RFC 9562 UUIDv7 generator added with rollback-safe time ordering, per-millisecond sequencing, version/variant validation, monotonicity, concurrency, and uniqueness tests.
- Thirty-day soft-delete cleanup application job skeleton added with injectable retention, SQL error handling, and tests.
- `scripts/test-postgres.sh` makes the no-Docker ephemeral PostgreSQL proof reproducible and cleans temporary files/processes.

## 2026-08-12 — repair round 4 (migration and entity channel hardening)

- PostgreSQL migration execution serialized by a transaction-scoped advisory lock; every pre-commit failure path verified to roll back and commit failures surfaced with migration context.
- Fresh-schema PostgreSQL integration test added behind `TEST_DATABASE_URL`, including idempotent history and case-insensitive Genre uniqueness; environment had no database URL or usable local Docker daemon, so the live integration case remains skipped here.
- Verification expanded from selected Go packages to `./...`, including race detection and all command builds.
- BroadcastChannel messages now receive strict runtime shape, UUID, version, and RFC 3339 validation before state mutation; valid `ENTITY_INVALIDATED` messages evict matching non-stale entities while invalid and stale messages are rejected.
- Entity-kind matching now prevents a snapshot or invalidation for one entity type from mutating another type that happens to use the same UUID; publishing no longer hard-codes `task`.
- Frontend P1 tests cover valid snapshots, invalidation, stale invalidation, cross-kind messages, malformed UUID/version/timestamp (including impossible calendar dates), nested/top-level extra properties, unknown types, and no-mutation rejection.
- Broadcast snapshot schema now rejects extra properties inside the nested entity object, matching strict runtime validation.

## 2026-08-12 — repair round 3 (migration runner)

- Embedded, ordered PostgreSQL migration runner with transactional application and durable, idempotent migration history.
- `cmd/migrate` entry point using `DATABASE_URL`; existing migrations remain immutable.
- Bootstrap rejects unsupported or unreadable Go versions before dependency installation.
- TDD coverage added for bootstrap ordering; migration behavior covered by package validation and final build/verification.

## 2026-08-12 — repair round 2

- Go module/API bootstrap; loopback-only listener and mandatory internal Binding secret.
- Shared Task FSM/completion invariant used by HTTP, MCP-callable application service, and Google completion adapter.
- Repository ownership checks and expected-version update semantics; canonical SQL predicate documented beside repository.
- Security tests for missing Binding context, identity override, cross-user access, and stale version.
- OpenAPI internal Binding and Worker user-context security schemes.
- Strict verification pipeline: format, vet/typecheck/static analysis, Go/Python tests, build, contract validation.
- `check_completion.py` now recognizes only structured blocker entries, not explanatory prose.
- Validation: gofmt clean; `go test ./...`, `go vet ./...`, `go build ./...`, Python contract tests passed with isolated Go 1.24.6 toolchain.
- Remaining unchecked checklist items are real future implementation scope; completion gate intentionally remains closed.

## 2026-08-12 — Phase 7 frontend repair

- Real Nuxt 3 package/application with core, shared UI, task, genre, dashboard, search, and Google layer boundaries.
- Responsive warm-dark design system from `/home/ichiken/DESIGN.md`; keyboard focus, skip link, semantic navigation/dialog/table/form markup, 44px mobile targets, reduced-motion support.
- Required routes, shared create/edit TaskEditor, hierarchy/dependency distinction, multi-dependency warning, fire state, dashboards, deadline sections, confirmations, Gantt, genre root progress, URL search state, per-page SEO titles.
- BroadcastChannel snapshots reject stale versions; message shape aligned with `broadcast/message.schema.json`.
- TDD: frontend behavior/structure tests added first and observed failing before implementation; 11 tests green.
- Verification gate now requires npm test/lint/typecheck/build and supports explicit `TASMANE_GO_BIN`; bootstrap uses `npm ci` and detects installed Go 1.22.12.
- Critical-fetch/API-backed prefetch remains unchecked because backend list/dashboard endpoints are not implemented; UI uses deterministic demo data rather than inventing transport behavior.
## 2026-08-13 — Phase 2 overdue/fire projection

- Added a transport-independent deadline projection that derives `isOverdue` and propagates `showFire` from any incomplete overdue descendant to its unfinished ancestors.
- Completed tasks and the exact deadline boundary do not show overdue/fire; malformed hierarchy snapshots (empty/duplicate IDs, missing parents, cycles) fail closed.
- Added focused domain tests, including propagation through a completed intermediary without displaying fire on that completed task.
- Verification: `go test ./internal/domain`.
## 2026-08-13 — Phase 2 date propagation impact calculator

- Added a transport-independent, deterministic date impact calculator for ancestor expansion and descendant contraction across both start and end boundaries.
- Plans include before/after values for every mutation, validate all date ranges and hierarchy references, and reject cycles instead of emitting partial effects.
- Added focused tests for child expansion, parent contraction across multiple levels, deterministic no-op filtering, and invalid input.
- Verification: `go test ./internal/domain`.
## 2026-08-13 — Phase 2 parent persistence boundary

- Added explicit parent metadata and a domain persistence projection that always clears `leaf_progress` for tasks with children while retaining parent status.
- Application status transitions now normalize at the repository boundary; completed-ancestor rollback persists `IN_PROGRESS` with NULL leaf progress rather than the derived display value 95.
- Added domain and application tests proving parent values are cleared and leaf values are preserved.
- Verification: `go test ./internal/domain ./internal/application`.
## 2026-08-13 — Phase 2 resolved decision policies

- Added tested domain plans for OD-001 genre subtree movement/boundary dependency removal, OD-002 Google due hierarchy conflict, OD-003 dedicated Google Task List create/reuse, OD-006 first-child confirmation/progress clearing, and OD-007 recursive deduplicated deletion impact.
- OD-005 remains enforced by the isolated scoring constants and exact-weight tests; OD-008 remains enforced by internal Binding authentication and contract tests.
- The new plans are transport-independent so later HTTP, Google queue, and MCP adapters must consume the same decisions.
- Verification: `go test ./internal/domain`.
## 2026-08-13 — Phase 3 PostgreSQL Task repository

- Added the production PostgreSQL Task repository with tenant-scoped active-row lookup, recursive incomplete-descendant and unresolved-Confirmation derivation, row locking, optimistic version enforcement, and parent leaf-progress normalization.
- Status and progress histories are generated as UUIDv7 and inserted in the same transaction as the Task mutation; injected history failure tests prove the Task update rolls back.
- Added a live fresh-schema repository test covering cross-user hiding, recursive state, parent transition persistence, version increment, and history creation.
- Hardened the ephemeral PostgreSQL harness by installing required extensions once in `public`, retaining isolated application schemas and deterministic sequential package execution.
- Verification: repository unit tests and `scripts/test-postgres.sh`.
## 2026-08-13 — Phase 3 Genre and TaskType repositories

- Added tenant-scoped PostgreSQL CRUD repositories for Genre and TaskType with trimmed required names, deterministic active lists, soft-delete hiding, and unique violations mapped to domain conflict.
- Genre deletion locks the resource and rejects active Tasks; TaskType detachment/deletion locks relations/resources and rejects active Task use.
- Genre/TaskType attachment verifies both active resources belong to the same authenticated user and is idempotent without allowing a cross-user no-op.
- Live fresh-schema tests cover normalized-name uniqueness, list/delete visibility, relation attachment, cross-user rejection, and in-use protections.
## 2026-08-13 — Phase 3 Confirmation repository

- Expanded Confirmation entities with persistence identity/content/due metadata while preserving the existing FSM and scoring behavior.
- Added active-Task/tenant-scoped create, get, and deterministic list operations; creation trims required fields and cannot attach to a foreign or deleted Task.
- FSM transitions carry their expected prior status into an atomic conditional update, converting concurrent transitions into `ErrVersionConflict` rather than silently overwriting state.
- Unit and live fresh-schema tests cover creation shape, shared Application transition, listing, and stale transition rejection.
## 2026-08-13 — Phase 3 Dependency repository

- Added active, tenant-scoped hierarchy/dependency graph loading and deterministic per-Task dependency listing.
- Parent and dependency writes acquire a per-user PostgreSQL advisory transaction lock, reload the graph inside the lock, and call the shared Domain cycle/ownership/Genre validators immediately before persistence.
- Duplicate dependency insertion is a domain conflict; missing/cross-user rows remain hidden; removal is owner-scoped.
- Unit tests prove repository-level revalidation and rollback, while live tests cover shared Application writes, persisted listing, cycle rejection, and cross-user isolation.
## 2026-08-13 — Phase 4 chi Task API and Impact confirmation

- Replaced manual path parsing with chi routing and connected real PostgreSQL-backed Task, Genre, TaskType, Dependency, and Confirmation use cases through the authenticated user context.
- Task create/get/search/tree/update/delete are implemented with lower-camel DTOs, optimistic versions, tenant/soft-delete isolation, root/subtree reads, derived parent progress, and overdue/fire projections.
- First-child conversion, date propagation, and recursive Task deletion use a shared 409 ImpactPlan flow with HMAC-SHA256 tokens bound to user, operation, resource version, and five-minute expiry.
- Confirmed mutations revalidate target versions and affected date before-values inside the transaction; stale state rolls back. Task deletion remains Web-only and is absent from MCP.
- API startup now requires PostgreSQL and both internal-binding and impact-token secrets, runs migrations, listens on loopback, and shuts down gracefully.
- Live PostgreSQL tests cover create, confirmed Leaf-to-Parent conversion, search/tree isolation, patch, recursive preview, and confirmed soft deletion.
## 2026-08-13 — Phase 4 dashboard routing and contract gate

- Added global and Genre-scoped dashboard routes using the shared tenant-scoped Task read model and the same progress/deadline projection as search and tree responses.
- All Phase 4 domain routes are registered with chi; Google-specific routes remain owned by the Phase 5 subsystem.
- OpenAPI/MCP contract validation and all contract unit tests pass against the implemented DTO/security invariants.
## 2026-08-13 — Phase 5 Google sync foundations

- Added AES-256-GCM refresh-token protection with authenticated tamper detection and PostgreSQL persistence that never accepts plaintext at its boundary.
- Added queue producer/consumer contracts and tested retry, refresh-and-retry, terminal failure, and sync-state transitions without blocking HTTP requests on Google calls.
- Added deterministic field snapshot merge/conflict logic and reused the hierarchy date planner to classify Google due changes requiring ancestor propagation as conflict.
- Added dedicated Tasmane list create/reuse policy and persistence, per-user lease acquisition, settings-driven access-time gate, sync status storage, and Google-only import candidate reads.
- Additive migration 0004 allows sync candidate records without a local Task; fresh migration and repository suites pass.
## 2026-08-13 — Phase 5 Google lifecycle policies

- Google due-date parsing preserves the local Task's timezone and hh:mm while rejecting malformed dates; ancestor extension remains a conflict through the shared date planner.
- Remote Google deletion is classified as CONFLICT and never deletes the local Task automatically.
- Google uncomplete uses the Task FSM and the PostgreSQL implementation atomically reopens the completed target and every completed ancestor, writing `GOOGLE_TASKS` status history for each mutation.
## 2026-08-13 — Phase 6 MCP server

- Added a stdio JSON-RPC MCP server supporting initialize, tools/list, and tools/call with a trusted per-process user context.
- Converted every manifest input definition to a strict MCP JSON Schema and added a test proving all 25 declared tools have implementations and none exposes delete.
- Tool calls invoke the same Task, Catalog, Relation, Confirmation, and Google Application use cases as HTTP; Domain completion/cycle/tenant rules are not duplicated or bypassed.
- Added a production MCP command backed by PostgreSQL and a durable Google sync job table/queue adapter via additive migration 0005.
- Fresh PostgreSQL and full Go race suites pass.
## 2026-08-13 — Phase 7 critical rendering and real API data

- Replaced static Task, Genre, Dashboard, Search, Gantt, and Top-page sample arrays with typed calls to the real same-origin API.
- Added a reusable critical-data composable: route-critical data is awaited/rendered first, then likely next-navigation data is prefetched after Vue's render tick.
- Task detail renders the Task before concurrently prefetching its hierarchy and dependencies; loading/error states use accessible live roles.
- Search retains URL/bookmark/back-forward state while discarding stale out-of-order responses; Genre filters come from the API.
- Warp-inspired layout/styles were preserved. Frontend 22 tests, lint, strict typecheck, and production build pass.
