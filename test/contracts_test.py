import json
from pathlib import Path
import unittest

class Contracts(unittest.TestCase):
    def test_broadcast_schema_is_strict_at_every_message_object_boundary(self):
        doc = json.loads(Path("broadcast/message.schema.json").read_text())
        for variant in doc["oneOf"]:
            self.assertFalse(variant["additionalProperties"])
        snapshot_entity = doc["oneOf"][0]["properties"]["entity"]
        self.assertFalse(snapshot_entity["additionalProperties"])

    def test_mcp_has_no_delete_and_updates_require_version(self):
        doc=json.loads(Path("mcp/tools.json").read_text())
        self.assertFalse(any(t["action"]=="delete" or "delete" in t["name"] for t in doc["tools"]))
        for tool in (t for t in doc["tools"] if t["name"] in {"update_task","transition_task"}):
            self.assertIn("expectedVersion", tool["input"])
    def test_openapi_auth_and_optimistic_contract(self):
        text=Path("api/openapi.yaml").read_text()
        self.assertIn("InternalBindingAuth", text)
        self.assertIn("WorkerUserContext", text)
        self.assertIn("security:", text)
        self.assertIn("expectedVersion", text)
    def test_sql_optimistic_predicate_is_owned(self):
        text=Path("internal/repository/memory.go").read_text()
        self.assertIn("user_id = $4 AND version = $5", text)

if __name__ == "__main__": unittest.main()
