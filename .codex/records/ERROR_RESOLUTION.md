# Error resolution

- `401 unauthorized`: verify Worker Binding secret and authenticated user injection.
- `404 forbidden resource`: authenticated user does not own the resource; do not disclose its existence.
- `409 version conflict`: refetch and retry with the current version after user reconciliation.
- `409 task is not completable`: resolve incomplete descendants or unresolved confirmations.
- `422 invalid task transition`: follow the Task FSM in the product specification.
- Error: a second fresh-schema integration package failed with `type "citext" does not exist` while applying `0001_init.sql`.
  Cause: PostgreSQL already registered the extension database-wide with its types in `public`, while test connections replaced `search_path` with only the isolated schema.
  Resolution: preserve immutable migrations; install both required extensions into `public` once when the ephemeral server starts, keep each isolated schema first with `public` in `search_path`, and run schema-owning package tests sequentially.
