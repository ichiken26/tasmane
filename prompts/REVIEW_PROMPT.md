# REVIEW PROMPT

変更差分を仕様正本と照合してレビューしてください。

重点:
1. Domain invariant漏れ
2. Transport間のロジック重複
3. Google/MCPによる制約迂回
4. PostgreSQL FK/CHECK/UNIQUE不足
5. optimistic locking漏れ
6. soft delete考慮漏れ
7. user isolation
8. dependency/hierarchy cycle
9. score仕様逸脱
10. OpenAPI/MCP contract drift
11. test不足

Critical / High / Medium / Low で出力し、Critical/Highが0でなければ完了扱いにしない。


At the very end of `state/review.md`, write exactly one of:
- `REVIEW_STATUS: PASS`
- `REVIEW_STATUS: FAIL`

PASS only if Critical=0 and High=0.
