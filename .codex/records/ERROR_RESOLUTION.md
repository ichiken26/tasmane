# Error resolution

- `401 unauthorized`: verify Worker Binding secret and authenticated user injection.
- `404 forbidden resource`: authenticated user does not own the resource; do not disclose its existence.
- `409 version conflict`: refetch and retry with the current version after user reconciliation.
- `409 task is not completable`: resolve incomplete descendants or unresolved confirmations.
- `422 invalid task transition`: follow the Task FSM in the product specification.
- Error: a second fresh-schema integration package failed with `type "citext" does not exist` while applying `0001_init.sql`.
  Cause: PostgreSQL already registered the extension database-wide with its types in `public`, while test connections replaced `search_path` with only the isolated schema.
  Resolution: preserve immutable migrations; install both required extensions into `public` once when the ephemeral server starts, keep each isolated schema first with `public` in `search_path`, and run schema-owning package tests sequentially.
- Error: the requested `local_builder` collaboration role was unavailable when starting the final Google runtime implementation.
  Cause: the current collaboration environment did not expose that role despite repository orchestration guidance.
  Resolution: start a bounded `worker` agent with the same backend ownership and no-overwrite constraints.
- Error: direct `gofmt` and `go test` failed because Go was absent from the inherited shell `PATH`.
  Cause: this checkout uses the verified repository-local toolchain at `/home/ichiken/.local/toolchains/go1.22.12/bin`.
  Resolution: set `TASMANE_GO_BIN` explicitly and prepend its directory for formatting and tests.
- Error: frontend typecheck temporarily failed in newly added Cloudflare OAuth utilities (`JsonWebKey.kid` and Web Crypto `BufferSource` types).
  Cause: the parallel gateway implementation was observed before its type-safe key wrapper and ArrayBuffer conversions were complete.
  Resolution: return the exact diagnostics to the gateway owner and keep the release gate red until lint, typecheck, tests, and build all pass.
