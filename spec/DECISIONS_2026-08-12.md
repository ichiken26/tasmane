# Decisions — 2026-08-12

This file records the final decisions for previously open items.

1. Genre move: warn and remove cross-genre boundary dependencies; move the entire subtree.
2. Google due vs hierarchy: mark CONFLICT and require user decision.
3. Google Tasks: automatically create a dedicated Tasmane task list.
4. Scoring: isolate tunable constants/functions so values can be adjusted later without rewriting algorithms.
5. Leaf->Parent: show progress recalculation confirmation; completed tasks cannot receive children.
6. Delete with dependencies: show recursive impact set; after confirmation soft-delete all listed affected tasks.
7. Auth boundary: Worker -> Go Container uses Cloudflare internal path/Tunnel + Binding; no direct public Go ingress.
