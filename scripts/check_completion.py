from pathlib import Path
import re, sys

# 完了判定用ステートファイルの読込
check = Path("state/implementation-checklist.md").read_text(encoding="utf-8")
blocked = Path("state/blocked.md").read_text(encoding="utf-8")
review_path = Path("state/review.md")

# 未完了チェックリスト項目の抽出
unchecked = re.findall(r"^- \[ \] (.+)$", check, re.M)

# P0 / P1 / BLOCKER / CRITICAL 行の抽出
blocking_lines = [line for line in blocked.splitlines()
                  if re.match(r"^\s*-\s+(P0|P1|BLOCKER|CRITICAL)\s*:", line, re.I)]

errors = []

if unchecked:
    errors.append(f"{len(unchecked)} checklist items remain")

if blocking_lines:
    errors.append("blockers remain:\n" + "\n".join(blocking_lines))

# 最終レビュー PASS と Critical/High 残件の確認
if not review_path.exists():
    errors.append("state/review.md does not exist")
else:
    review = review_path.read_text(encoding="utf-8")
    if "REVIEW_STATUS: PASS" not in review:
        errors.append("final review is not PASS")
    if re.search(r"(?im)^\s*(Critical|High)\s*:\s*[1-9]\d*", review):
        errors.append("Critical/High findings remain")

# 完了ゲート未充足時の集約出力
if errors:
    print("[completion] NOT COMPLETE")
    for e in errors:
        print(e)
    sys.exit(1)

print("[completion] gates satisfied")
