# P2P Runtime Guide

本文描述本仓库内置的低延迟 P2P 协作运行时（Raft 共识 + 确定性状态机）的启动和调用方式。

## 1. 组件

- 命令入口: `cmd/p2pnode/main.go`
- 协议层: `internal/p2p/protocol`
- 状态机: `internal/p2p/state`
- 共识层: `internal/p2p/consensus`
- HTTP API: `internal/p2p/api`

## 2. 环境变量

- `P2P_NODE_ID`: 节点 ID，例如 `node-1`
- `P2P_RAFT_ADDR`: Raft 复制地址，例如 `127.0.0.1:17000`
- `P2P_HTTP_ADDR`: HTTP 地址，例如 `127.0.0.1:18080`
- `P2P_DATA_DIR`: 数据目录，默认 `tmp/p2pnode/<node_id>`
- `P2P_BOOTSTRAP`: 是否引导新集群（`true/false`）
- `P2P_JOIN_ENDPOINT`: 非引导节点自动加入地址，如 `http://127.0.0.1:18080`
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

- `GET /healthz`: 节点健康与 leader 信息
- `POST /v1/p2p/tx`: 提交已签名事务（必须到 leader）
- `GET /v1/p2p/stats`: 状态统计
- `GET /v1/p2p/raft`: Raft 运行状态
- `POST /v1/p2p/raft/join`: leader 添加节点
- `POST /v1/p2p/raft/remove`: leader 移除节点
- `GET /v1/p2p/sessions/{sessionId}`
- `GET /v1/p2p/sessions/{sessionId}/participants`
- `GET /v1/p2p/sessions/{sessionId}/steps/open`
- `GET /v1/p2p/sessions/{sessionId}/events`
- `GET /v1/p2p/steps/{stepId}`
- `GET /v1/p2p/steps/{stepId}/artifacts`

## 5. 写入约束

- 所有写入均以 `protocol.Tx` 形式提交。
- `Tx` 必须带 `public_key` 和 `signature`，节点会在共识前后执行验签和业务校验。
- follower 会返回 `NOT_LEADER`，响应中包含 leader 地址提示。

## 6. 当前能力边界

- 这是最小可运行版 P2P Runtime，已覆盖会话、参与者、认领、移交、产物、决策、投票、步骤收敛等核心流程。
- 事务签名密钥分发、跨节点发现优化、gossip 数据面、NAT 穿透等仍需在后续版本继续增强。

## 7. 事务类型（op）

- `SESSION_CREATE`
- `PARTICIPANT_JOIN`
- `STEP_CLAIM`
- `STEP_RELEASE`
- `STEP_HANDOFF`
- `ARTIFACT_ADD`
- `DECISION_OPEN`
- `VOTE_CAST`
- `STEP_RESOLVE`

字段定义参考 `internal/p2p/protocol/types.go`，签名构造可参考 `internal/p2p/protocol/types_test.go`。
