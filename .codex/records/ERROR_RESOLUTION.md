# Error resolution

- `401 unauthorized`: verify Worker Binding secret and authenticated user injection.
- `404 forbidden resource`: authenticated user does not own the resource; do not disclose its existence.
- `409 version conflict`: refetch and retry with the current version after user reconciliation.
- `409 task is not completable`: resolve incomplete descendants or unresolved confirmations.
- `422 invalid task transition`: follow the Task FSM in the product specification.
