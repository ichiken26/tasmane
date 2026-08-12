# Implementation Checklist

## Phase 0 Foundation
- [x] Go API project initialized
- [x] Nuxt project initialized
- [x] local dev env documented
- [x] migration runner configured
- [x] test commands wired into scripts/verify.sh

## Phase 1 Database
- [x] 0001 migration applies to fresh PostgreSQL
- [x] schema integration tests
- [x] UUID v7 generation policy
- [x] updated_at policy
- [x] soft-delete cleanup job skeleton

## Phase 2 Domain
- [x] Task FSM
- [x] CompleteTask invariants
- [x] Parent progress
- [x] Parent rollback propagation
- [x] Hierarchy cycle detection
- [x] Dependency cycle detection
- [x] Priority calculator
- [x] TaskScore calculator + reasons
- [x] Confirmation FSM/score
- [x] Overdue/fire projection
- [x] Date propagation impact calculator
- [x] Parent persistence uses status + derived progress (no parent leaf_progress)
- [x] Resolved OD-001/002/003/005/006/007/008 behaviors covered by tests

## Phase 3 Repository / UseCases
- [x] Task repository
- [x] Genre repository
- [x] TaskType repository
- [x] Confirmation repository
- [x] Dependency repository
- [x] history write atomicity
- [x] optimistic lock

## Phase 4 HTTP
- [ ] chi routes
- [x] injected user context middleware
- [x] Worker Binding/internal-only Go ingress
- [x] DTO validation
- [x] domain error mapping
- [x] ImpactPlan preview/confirm protocol
- [ ] OpenAPI conformance

## Phase 5 Google Tasks
- [ ] encrypted token persistence abstraction
- [ ] queue producer
- [ ] queue consumer
- [ ] retry classification
- [ ] field snapshot conflict detection
- [ ] Google due hierarchy conflict => CONFLICT
- [ ] dedicated Tasmane Google Task List create/reuse
- [ ] dedicated Google Task List ID persistence
- [ ] merge
- [x] Google complete via domain
- [ ] Google uncomplete via domain
- [ ] deletion conflict
- [ ] import candidate
- [ ] per-user sync lock
- [ ] access-time sync gate

## Phase 6 MCP
- [x] OpenAPI/MCP contract completeness
- [ ] MCP server
- [ ] tool schemas
- [ ] shared usecases
- [x] no delete tools
- [x] tests prove no delete exposure

## Phase 7 Frontend
- [x] layout/header/sidebar
- [x] TaskEditor create/edit
- [x] hierarchy tree
- [x] dependency reference visualization
- [x] yellow multi-dependency highlight
- [x] fire propagation UI
- [x] dashboard
- [x] genre dashboard
- [x] search URL state
- [x] Gantt
- [x] /genres root-task progress view
- [x] dashboard deadline sections
- [x] confirmation recommendation list
- [x] page titles / Nuxt SEO meta
- [x] Nuxt Layers boundaries
- [x] date propagation warning modal
- [ ] critical fetch -> render -> prefetch
- [x] BroadcastChannel version handling
- [x] BroadcastChannel JSON schema conformance

## Phase 8 AI
- [ ] Gemini provider
- [ ] 04:00 timezone scheduler
- [ ] whole-dashboard advice
- [ ] genre advice
- [ ] source hash
- [ ] 30-day retention

## Phase 9 Release gates
- [ ] all P0/P1 tests pass
- [ ] review Critical/High = 0
- [ ] security/user-isolation review
- [ ] migration reproducible on fresh DB
- [ ] README operational steps
