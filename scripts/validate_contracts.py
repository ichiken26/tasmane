from pathlib import Path
import json, re, sys

errors = []

# 契約検証対象ファイルの読込
spec = Path("spec/PRODUCT_SPEC.md").read_text(encoding="utf-8")
mcp = json.loads(Path("mcp/tools.json").read_text(encoding="utf-8"))
sql = Path("db/migrations/0001_init.sql").read_text(encoding="utf-8")
openapi = Path("api/openapi.yaml").read_text(encoding="utf-8")

# MCP からの delete ツール公開の禁止
if any(t.get("action") == "delete" or "delete" in t.get("name","").lower() for t in mcp["tools"]):
    errors.append("MCP must not expose delete tool")

# 必須テーブル名のスキーマ存在確認
for token in ["task_status", "task_dependencies", "google_task_syncs", "user_server_settings"]:
    if token not in sql:
        errors.append(f"schema missing {token}")

# 仕様書の MCP 削除ポリシー記載確認
if "MCP" not in spec or not ("delete" in spec.lower() or "削除" in spec):
    errors.append("spec missing MCP delete policy")

# OpenAPI のタスク遷移エンドポイント確認
if "/v1/tasks/{taskId}/transition" not in openapi:
    errors.append("openapi missing task transition endpoint")

# Parent 進捗の導出不変条件のスキーマ記載確認
if "Parent progress is always derived" not in sql:
    errors.append("schema must document derived Parent progress invariant")

# タイムゾーン永続化の仕様またはスキーマ確認
if "users.timezone" not in spec and "timezone" not in sql:
    errors.append("timezone persistence missing")

# 必須 UI ルートの仕様記載確認
for route in ["/dashboard", "/tasks/new", "/search"]:
    if route not in spec:
        errors.append(f"product spec missing UI route {route}")



# Google Tasks 専用リスト ID の永続化確認
if "google_task_list_id text" not in sql:
    errors.append("google_integrations must persist dedicated google_task_list_id")

# OpenAPI 必須 operationId の存在確認
required_ops = [
    "getGenreDashboard", "updateConfirmation", "listTaskDependencies",
    "removeTaskDependency", "getUserSettings", "updateUserSettings",
    "listGoogleConflicts", "listGoogleImportCandidates"
]
for op in required_ops:
    if op not in openapi:
        errors.append(f"openapi missing operation {op}")

# MCP 必須ツール名の存在確認
required_mcp = [
    "list_genres", "create_genre", "update_genre",
    "list_task_types", "create_task_type", "update_task_type",
    "get_dashboard", "get_genre_dashboard"
]
for tool in required_mcp:
    if not any(t.get("name") == tool for t in mcp["tools"]):
        errors.append(f"mcp missing tool {tool}")

# BroadcastChannel メッセージスキーマの存在確認
if not Path("broadcast/message.schema.json").exists():
    errors.append("BroadcastChannel message schema missing")

# 契約違反の集約出力
if errors:
    print("\n".join(f"ERROR: {e}" for e in errors))
    sys.exit(1)

print("contract validation OK")
