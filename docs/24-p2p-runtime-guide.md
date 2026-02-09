# P2P Runtime Guide

本文描述当前仓库内置的 P2P 协作运行时（Raft 共识 + 确定性状态机）的启动与调用方式。

## 1. 组件
- 命令入口: `cmd/p2pnode/main.go`
- 协议定义: `internal/p2p/protocol/types.go`
- 状态机: `internal/p2p/state/state.go`
- 共识层: `internal/p2p/consensus/node.go`
- HTTP API: `internal/p2p/api/server.go`
- 事务签名构造器: `scripts/p2p-txgen.go`

## 2. 环境变量
- `P2P_NODE_ID`: 节点 ID，例如 `node-1`
- `P2P_RAFT_ADDR`: Raft 地址，例如 `127.0.0.1:17000`
- `P2P_HTTP_ADDR`: HTTP 地址，例如 `127.0.0.1:18080`
- `P2P_DATA_DIR`: 数据目录，默认 `tmp/p2pnode/<node_id>`
- `P2P_BOOTSTRAP`: 是否引导新集群（`true/false`）
- `P2P_JOIN_ENDPOINT`: 非引导节点加入入口，例如 `http://127.0.0.1:18080`
- `P2P_JOIN_RETRIES`: 自动加入重试次数，默认 `30`
- `P2P_JOIN_RETRY_DELAY`: 自动加入重试间隔，默认 `1s`
- `P2P_APPLY_TIMEOUT`: 事务应用超时，默认 `5s`

## 3. 启动方式
启动第一个节点（引导节点）:

```powershell
$env:P2P_NODE_ID = "node-1"
$env:P2P_RAFT_ADDR = "127.0.0.1:17000"
$env:P2P_HTTP_ADDR = "127.0.0.1:18080"
$env:P2P_BOOTSTRAP = "true"
go run ./cmd/p2pnode
```

启动第二个节点并加入集群:

```powershell
$env:P2P_NODE_ID = "node-2"
$env:P2P_RAFT_ADDR = "127.0.0.1:17001"
$env:P2P_HTTP_ADDR = "127.0.0.1:18081"
$env:P2P_JOIN_ENDPOINT = "http://127.0.0.1:18080"
go run ./cmd/p2pnode
```

## 4. 关键接口
- `GET /healthz`: 健康状态与 leader 信息
- `GET /v1/p2p/raft`: Raft 状态
- `POST /v1/p2p/tx`: 提交签名事务（必须发给 leader）
- `GET /v1/p2p/stats`: 状态统计
- `GET /v1/p2p/sessions/{sessionId}`
- `GET /v1/p2p/sessions/{sessionId}/participants`
- `GET /v1/p2p/sessions/{sessionId}/steps/open`
- `GET /v1/p2p/sessions/{sessionId}/events`
- `GET /v1/p2p/steps/{stepId}`
- `GET /v1/p2p/steps/{stepId}/artifacts`

## 5. 使用 p2p-txgen 生成签名事务
`p2p-txgen` 会输出完整 `protocol.Tx` JSON 到 stdout。

基础调用:

```powershell
go run ./scripts/p2p-txgen.go --op session-create --session-id demo --actor user:admin
```

发送到 leader:

```powershell
$tx = go run ./scripts/p2p-txgen.go --op session-create --session-id demo --actor user:admin
Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18080/v1/p2p/tx" -Body $tx -ContentType "application/json"
```

通用参数:
- `--op`: 事务类型
- `--session-id`: 会话 ID（部分 op 必填）
- `--actor`: 行为主体
- `--tx-id`: 自定义 tx_id
- `--nonce`: 自定义 nonce
- `--timestamp`: RFC3339 时间戳
- `--private-key`: base64 私钥（32 字节 seed 或 64 字节私钥）

支持的 `op` 与关键参数:
- `session-create`: `--session-id`，可选 `--session-name --workflow-id --context-json --steps-json`
- `participant-join`: `--session-id --participant-id`，可选 `--participant-type --participant-ref --participant-capabilities --trust-score`
- `step-claim`: `--step-id --participant-id`，可选 `--claim-id --lease-seconds`
- `step-release`: `--step-id --participant-id`
- `step-handoff`: `--step-id --from-participant-id --to-participant-id`，可选 `--new-claim-id --lease-seconds --comment`
- `artifact-add`: `--step-id --producer-id`（或 `--participant-id` 兜底），且必须提供 `--content-json` 或 `--external-uri`
- `decision-open`: `--step-id`，可选 `--decision-id --policy-json --deadline`
- `vote-cast`: `--decision-id --participant-id`，可选 `--vote-id --choice --comment`
- `step-resolve`: `--step-id`，可选 `--participant-id`

## 6. 各 op 示例
`SESSION_CREATE`:

```powershell
go run ./scripts/p2p-txgen.go --op session-create --session-id c-compiler --session-name "C Compiler" --steps-json "[{\"step_id\":\"lex\",\"step_key\":\"lexer\",\"name\":\"Lexer\",\"required_capabilities\":[\"lexer\"]}]"
```

`PARTICIPANT_JOIN`:

```powershell
go run ./scripts/p2p-txgen.go --op participant-join --session-id c-compiler --participant-id p-lexer --participant-type AGENT --participant-ref agent:lexer --participant-capabilities lexer
```

`STEP_CLAIM`:

```powershell
go run ./scripts/p2p-txgen.go --op step-claim --step-id lex --participant-id p-lexer --claim-id claim-lex-1 --lease-seconds 600
```

`STEP_RELEASE`:

```powershell
go run ./scripts/p2p-txgen.go --op step-release --step-id lex --participant-id p-lexer
```

`STEP_HANDOFF`:

```powershell
go run ./scripts/p2p-txgen.go --op step-handoff --step-id lex --from-participant-id p-lexer --to-participant-id p-review --new-claim-id claim-review-1 --comment "handoff for review"
```

`ARTIFACT_ADD`:

```powershell
go run ./scripts/p2p-txgen.go --op artifact-add --step-id lex --producer-id p-review --artifact-id artifact-lex-1 --artifact-kind report --content-json "{\"summary\":\"lexer passed\"}"
```

`DECISION_OPEN`:

```powershell
go run ./scripts/p2p-txgen.go --op decision-open --step-id lex --decision-id decision-lex-1 --policy-json "{\"min_approvals\":1}"
```

`VOTE_CAST`:

```powershell
go run ./scripts/p2p-txgen.go --op vote-cast --decision-id decision-lex-1 --participant-id p-review --vote-id vote-1 --choice APPROVE
```

`STEP_RESOLVE`:

```powershell
go run ./scripts/p2p-txgen.go --op step-resolve --step-id lex --participant-id p-review
```
