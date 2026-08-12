from pathlib import Path
import re, sys

check = Path("state/implementation-checklist.md").read_text(encoding="utf-8")
blocked = Path("state/blocked.md").read_text(encoding="utf-8")
review_path = Path("state/review.md")

unchecked = re.findall(r"^- \[ \] (.+)$", check, re.M)

blocking_lines = [line for line in blocked.splitlines()
                  if re.match(r"^\s*-\s+(P0|P1|BLOCKER|CRITICAL)\s*:", line, re.I)]

errors = []

if unchecked:
    errors.append(f"{len(unchecked)} checklist items remain")

if blocking_lines:
    errors.append("blockers remain:\n" + "\n".join(blocking_lines))

if not review_path.exists():
    errors.append("state/review.md does not exist")
else:
    review = review_path.read_text(encoding="utf-8")
    if "REVIEW_STATUS: PASS" not in review:
        errors.append("final review is not PASS")
    if re.search(r"(?im)^\s*(Critical|High)\s*:\s*[1-9]\d*", review):
        errors.append("Critical/High findings remain")

if errors:
    print("[completion] NOT COMPLETE")
    for e in errors:
        print(e)
    sys.exit(1)

print("[completion] gates satisfied")
