from pathlib import Path
import json, re, sys

errors = []

spec = Path("spec/PRODUCT_SPEC.md").read_text(encoding="utf-8")
mcp = json.loads(Path("mcp/tools.json").read_text(encoding="utf-8"))
sql = Path("db/migrations/0001_init.sql").read_text(encoding="utf-8")
openapi = Path("api/openapi.yaml").read_text(encoding="utf-8")

if any(t.get("action") == "delete" or "delete" in t.get("name","").lower() for t in mcp["tools"]):
    errors.append("MCP must not expose delete tool")

for token in ["task_status", "task_dependencies", "google_task_syncs", "user_server_settings"]:
    if token not in sql:
        errors.append(f"schema missing {token}")

if "MCP" not in spec or not ("delete" in spec.lower() or "削除" in spec):
    errors.append("spec missing MCP delete policy")

if "/v1/tasks/{taskId}/transition" not in openapi:
    errors.append("openapi missing task transition endpoint")

if "Parent progress is always derived" not in sql:
    errors.append("schema must document derived Parent progress invariant")

if "users.timezone" not in spec and "timezone" not in sql:
    errors.append("timezone persistence missing")

for route in ["/dashboard", "/tasks/new", "/search"]:
    if route not in spec:
        errors.append(f"product spec missing UI route {route}")



if "google_task_list_id text" not in sql:
    errors.append("google_integrations must persist dedicated google_task_list_id")

required_ops = [
    "getGenreDashboard", "updateConfirmation", "listTaskDependencies",
    "removeTaskDependency", "getUserSettings", "updateUserSettings",
    "listGoogleConflicts", "listGoogleImportCandidates"
]
for op in required_ops:
    if op not in openapi:
        errors.append(f"openapi missing operation {op}")

required_mcp = [
    "list_genres", "create_genre", "update_genre",
    "list_task_types", "create_task_type", "update_task_type",
    "get_dashboard", "get_genre_dashboard"
]
for tool in required_mcp:
    if not any(t.get("name") == tool for t in mcp["tools"]):
        errors.append(f"mcp missing tool {tool}")

if not Path("broadcast/message.schema.json").exists():
    errors.append("BroadcastChannel message schema missing")

if errors:
    print("\n".join(f"ERROR: {e}" for e in errors))
    sys.exit(1)

print("contract validation OK")
