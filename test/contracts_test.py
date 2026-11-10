import json
from pathlib import Path
import unittest

# 契約ファイルの静的検証
class Contracts(unittest.TestCase):
    # BroadcastChannel 各メッセージ境界の additionalProperties 禁止
    def test_broadcast_schema_is_strict_at_every_message_object_boundary(self):
        doc = json.loads(Path("broadcast/message.schema.json").read_text())
        for variant in doc["oneOf"]:
            self.assertFalse(variant["additionalProperties"])
        snapshot_entity = doc["oneOf"][0]["properties"]["entity"]
        self.assertFalse(snapshot_entity["additionalProperties"])

    # MCP の delete 非公開と更新時 expectedVersion 必須
    def test_mcp_has_no_delete_and_updates_require_version(self):
        doc=json.loads(Path("mcp/tools.json").read_text())
        self.assertFalse(any(t["action"]=="delete" or "delete" in t["name"] for t in doc["tools"]))
        for tool in (t for t in doc["tools"] if t["name"] in {"update_task","transition_task"}):
            self.assertIn("expectedVersion", tool["input"])
    # OpenAPI の内部 Binding 認証と楽観ロック契約
    def test_openapi_auth_and_optimistic_contract(self):
        text=Path("api/openapi.yaml").read_text()
        self.assertIn("InternalBindingAuth", text)
        self.assertIn("WorkerUserContext", text)
        self.assertIn("security:", text)
        self.assertIn("expectedVersion", text)
    # 楽観ロック述語のリポジトリ所有
    def test_sql_optimistic_predicate_is_owned(self):
        text=Path("internal/repository/memory.go").read_text()
        self.assertIn("user_id = $4 AND version = $5", text)

if __name__ == "__main__": unittest.main()
