# TDD Test Plan

## P0 Domain tests

### Task FSM
- NOT_STARTED -> IN_PROGRESS succeeds
- IN_PROGRESS -> COMPLETED succeeds only when completable
- IN_PROGRESS -> NOT_STARTED sets progress NULL
- COMPLETED -> IN_PROGRESS sets progress 95
- NOT_STARTED -> COMPLETED rejected
- COMPLETED -> NOT_STARTED rejected

### CompleteTask invariants
- incomplete descendant => reject
- non-CONFIRMED confirmation => reject
- Google/MCP/REST all call same invariant

### Parent progress
- weighted average HIGH=5/MEDIUM=3/LOW=1
- uses effectivePriority
- non-completed parent clamps to 95
- completed parent = 100

### Parent rollback propagation
- child completed->in_progress rolls completed ancestors to 95
- parent explicit completed->in_progress does not reopen children
- parent in_progress->not_started resets in-progress descendants

### Hierarchy
- same user required
- same genre required
- cycle rejected
- self parent rejected

### Dependencies
- many-to-many
- same genre required
- cycle rejected
- indirect block scoring decays by distance
- multiple dependencies cause UI flag
- incomplete dependency warns but does not hard-block transition

### Genre movement
- subtree moves together
- all descendants change to destination genre
- old parent detached
- cross-boundary dependencies are detected before mutation
- cancel => no genre/dependency changes
- confirm => boundary dependencies removed and subtree moved atomically

### Priority
- threshold 0..34 LOW / 35..64 MEDIUM / 65..100 HIGH
- deadline/progress/dependency included
- no end => MEDIUM
- parent no end => nearest descendant end used

### TaskScore
- 0..100
- weight composition 30/25/20/15/5/5
- completed excluded
- no end => deadline urgency 50
- low progress raises score
- HIGH effective priority stronger
- dependency blocker score decays/caps
- age from createdAt with cap
- CHECKING strongly lowers score
- overdue+CHECKING lowers less than normal CHECKING
- incomplete dependency strongly lowers but item remains rankable
- tie => nearer deadline first

### Overdue
- incomplete own overdue => fire
- completed => no fire
- descendant overdue => ancestors show fire

### Confirmation
- adjacent transitions only
- direct UNCONFIRMED<->CONFIRMED rejected
- any unresolved confirmation blocks Task completion
- ConfirmationScore = due 50/status20/parentTask30
- deadline-less CHECKING fixed score > UNCONFIRMED fixed score

## P0 DB integration tests

- case-insensitive unique genre
- case-insensitive unique task type
- task requires valid genre
- task_type must be mapped to task genre
- parent same genre/user enforced
- dependency same genre/user enforced
- invalid progress/status combinations rejected
- optimistic lock updates one row only
- soft delete records deleted_at
- Google sync unique per user/task

## P0 Google sync contract tests

- Tasmane update responds before Google call
- queue message generated
- 429/5xx retries
- 401/403 refresh branch
- permanent 400 fails
- same-field bilateral change => CONFLICT
- different-field change => merge
- Google complete routes through CompleteTask
- domain-blocked Google complete => CONFLICT
- Google uncomplete reopens ancestors
- Google due date preserves local hh:mm
- Google due requiring ancestor extension => CONFLICT
- dedicated Tasmane Google Task List is created/reused
- Google delete => CONFLICT
- Tasmane delete => Google delete job
- restore => Google recreate job

## P1 HTTP/OpenAPI

- version mismatch => 409
- invalid FSM transition => 422
- domain conflict => 409
- request user id cannot override injected identity
- MCP has no delete tool

## P1 Frontend

- critical fetch renders before prefetch
- BroadcastChannel only accepts newer version
- dependency multi-parent reference highlighted
- ancestor fire visible while collapsed
- Gantt drag/resize semantics
- date propagation warning shows full impact set


## P0 Persistence correctness

- Parent task may persist status while `leaf_progress IS NULL`
- Parent calculated progress is never persisted into `leaf_progress`
- Leaf Domain invariant requires progress according to status
- first child addition to non-completed Leaf requires confirmation
- confirmed Leaf->Parent clears leaf_progress and switches to derived progress
- completed Leaf rejects child addition
- Parent->Leaf normalizes progress to status-compatible value

## P0 Soft delete / dependency

- deleted tasks do not participate in active TaskScore / blockers
- deleting a depended-on task computes recursive affected downstream tasks
- hierarchy descendants are included in delete impact
- duplicate impact paths are deduplicated
- cancel => nothing soft-deleted
- confirm => entire displayed impact set soft-deleted atomically
- subtree hard-delete cleanup can delete descendants before ancestors without FK failure

## P0 Authentication boundary

- user identity from request body/query/header cannot override Worker-authenticated context
- missing/unverifiable Worker context is rejected
- Go Container has no intended direct public ingress
- Worker Binding/internal path is the only application ingress to Go
- cross-user task/genre/taskType/dependency access rejected

## P1 UI/Application

- `/genres` shows Genre + Root Task progress
- dashboard deadline sections
- genre dashboard renders Gantt/AI Advice/confirmations
- TaskEditor create/edit reuse contract
- parent picker updates after Genre selection
- page titles
- sidebar current Genre highlight
- search query state restores from direct URL
- search updates with debounce for q
- task links can open new tab
- critical data renders before prefetch completes

## P0 Scoring configuration

- scoring constants are isolated from calculation algorithms
- weight totals are validated
- priority thresholds remain LOW 0..34 / MEDIUM 35..64 / HIGH 65..100
- changing a tuning constant changes only the intended component behavior

## P0 Google integration persistence

- dedicated list id persists in google_integrations.google_task_list_id
- reconnect reuses available dedicated list
- missing/deleted dedicated list enters recovery state

## P1 API/MCP contract completeness

- impact confirmation flow returns ImpactPlan before destructive/cascading mutation
- stale impactToken cannot be applied after version/state change
- MCP exposes no delete tool
- MCP covers required non-delete read/create/update use cases
- OpenAPI declares response schemas for core Task resources
- BroadcastChannel rejects older entity versions
