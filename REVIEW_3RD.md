# タスマネ Harness v3 Review

## 今回見つかった抜け

### Critical
1. Google専用Task Listを永続化すると仕様にあるのに、`google_integrations`にList ID列がなかった。
2. `api/openapi.yaml`が正本仕様に対して骨格だけで、影響確認・Genre Dashboard・Google conflict/import等の契約が欠落していた。
3. MCPがTask中心の一部Toolしかなく、「削除以外の取得・作成・更新をMCP/WebMCPでも利用」の仕様を満たしていなかった。

### High
4. Warning Modalを伴う変更のServer-side Impact Plan/再確認Protocolが未定義で、FrontendがDomain影響範囲を推測する余地があった。
5. BroadcastChannelのMessage Schemaがなかった。
6. `task_confirmations`が未確認状態でも`confirmed_at`を保持できるDB制約だった。
7. v2レビュー文書に解決済みOpen Decisionが「未確定」と残っており、Agentが誤読する可能性があった。

## v3で修正
- DDLへ`google_integrations.google_task_list_id`
- Confirmation DB CHECK強化
- OpenAPI拡張
- MCP Tool一覧拡張（DELETEは引き続き無し）
- Impact Preview/Confirm Contract
- BroadcastChannel JSON Schema
- Test Plan/Checklist/Contract Validator強化
- stale reviewをhistorical扱いに変更

## 残る注意点
Cloudflare ingressについて、要件の本質は「Go Containerへ直接Public ingressを持たずWorker経由に限定」。
Cloudflare-managed Containersではnative Worker/Container bindingが基本経路であり、Cloudflare Tunnel/VPCは外部/private infrastructureへ接続する用途でも使われるため、最終Wrangler構成では不要なTunnelを追加しないようCloudflare公式構成を確認して実装すること。
