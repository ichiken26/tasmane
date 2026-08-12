#!/usr/bin/env bash
set -euo pipefail

: "${AGENT_CMD:?Set AGENT_CMD. Example: export AGENT_CMD='codex exec --full-auto -'}"

MAX_ITERATIONS="${MAX_ITERATIONS:-100}"
SLEEP_SECONDS="${SLEEP_SECONDS:-2}"

mkdir -p state/logs

for ((i=1; i<=MAX_ITERATIONS; i++)); do
  echo "============================================================"
  echo "Tasmane TDD iteration $i / $MAX_ITERATIONS"
  echo "============================================================"

  PROMPT="$(cat prompts/MASTER_PROMPT.md)

## Current iteration
Iteration: $i

Read current repository, checklist, progress and blocked state. Execute exactly one coherent TDD iteration now.
Do not only describe work: edit files, run tests, and update state.
"

  LOG="state/logs/iteration-$(printf '%03d' "$i").log"

  # shellcheck disable=SC2086
  if ! printf '%s\n' "$PROMPT" | bash -lc "$AGENT_CMD" 2>&1 | tee "$LOG"; then
    echo "[loop] agent command exited non-zero; continuing to verification/recovery"
  fi

  if ./scripts/verify.sh; then
    # Completion requires a fresh independent review result.
    if [[ -n "${REVIEW_AGENT_CMD:-${AGENT_CMD:-}}" ]]; then
      ./scripts/review-loop.sh || true
    fi
    if python3 scripts/check_completion.py; then
      echo "[loop] completion gates passed"
      exit 0
    fi
  fi

  sleep "$SLEEP_SECONDS"
done

echo "[loop] max iterations reached without satisfying completion gates"
exit 2
