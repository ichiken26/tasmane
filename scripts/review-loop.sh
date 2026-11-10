#!/usr/bin/env bash
set -euo pipefail
# レビュー用エージェントコマンドの必須化
: "${REVIEW_AGENT_CMD:=${AGENT_CMD:-}}"
: "${REVIEW_AGENT_CMD:?Set REVIEW_AGENT_CMD or AGENT_CMD}"

# レビュープロンプトの組み立て
PROMPT="$(cat prompts/REVIEW_PROMPT.md)

Read spec/PRODUCT_SPEC.md, spec/OPEN_DECISIONS.md, current git diff, tests, DB schema, OpenAPI and MCP schemas.
Write review findings into state/review.md.
Do not modify implementation in this review invocation.
"

# レビューエージェントへのプロンプト投入
printf '%s\n' "$PROMPT" | bash -lc "$REVIEW_AGENT_CMD"
