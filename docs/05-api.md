# API 合同（当前实现）

本文档描述仓库内当前可用的 HTTP 接口，按实际代码路由整理。

## 认证规则
- `/v1/auth/bootstrap`、`/v1/auth/login` 无需登录。
- 其余 `/v1/*`、`/v2/*` 默认需要登录（Cookie 或 Bearer）。
- `X-Actor` 可用于显式声明执行主体（如 `agent:writer`）。

## V1 基础能力

### Auth
- `POST /v1/auth/bootstrap`
- `POST /v1/auth/login`
- `POST /v1/auth/logout`
- `GET /v1/auth/me`

### Users / Agents
- `POST /v1/users`（Admin）
- `GET /v1/users`（Admin）
- `GET /v1/users/{userId}`
- `PATCH /v1/users/{userId}`（Admin）
- `PUT /v1/users/{userId}/password`（Admin）
- `POST /v1/agents`
- `GET /v1/agents`

### Workflow
- `POST /v1/workflows`
- `GET /v1/workflows`
- `GET /v1/workflows/{workflowId}`
- `GET /v1/workflows/{workflowId}/versions`
- `POST /v1/workflows/{workflowId}/activate`
- `POST /v1/workflows/{workflowId}/deactivate`

### Executors
- `POST /v1/executors`
- `GET /v1/executors`
- `GET /v1/executors/{executorId}`
- `POST /v1/executors/{executorId}/activate`
- `POST /v1/executors/{executorId}/deactivate`

### Notifications
- `GET /v1/notifications`
- `GET /v1/notifications/{notificationId}`
- `POST /v1/notifications/{notificationId}/send`
- `GET /v1/notifications/sse`

### Trust / Audit
- `POST /v1/trust/events`
- `GET /v1/trust/evidence/{bundleType}/{subjectId}`
- `GET /v1/admin/audit`
- `GET /v1/admin/audit/{auditId}`

## V1 已移除接口
以下旧执行链路已下线，访问会返回 `410 GONE`：
- `/v1/tasks/*`
- `/v1/actions/*`
- `/v1/approvals/*`

## V2 协作运行时（中心化后端）

### Session
- `POST /v2/collab/sessions`
- `GET /v2/collab/sessions/{sessionId}`
- `POST /v2/collab/sessions/{sessionId}/join`
- `GET /v2/collab/sessions/{sessionId}/participants`
- `GET /v2/collab/sessions/{sessionId}/steps/open`
- `GET /v2/collab/sessions/{sessionId}/events`

### Step
- `POST /v2/collab/steps/{stepId}/claim`
- `POST /v2/collab/steps/{stepId}/release`
- `POST /v2/collab/steps/{stepId}/handoff`
- `POST /v2/collab/steps/{stepId}/artifacts`
- `GET /v2/collab/steps/{stepId}/artifacts`
- `POST /v2/collab/steps/{stepId}/decisions`
- `POST /v2/collab/steps/{stepId}/resolve`
- `GET /v2/collab/steps/{stepId}`

### Decision / Stream
- `POST /v2/collab/decisions/{decisionId}/votes`
- `GET /v2/collab/stream`

## P2P 运行时 API（`cmd/p2pnode`）

说明：这组接口由独立进程 `go run ./cmd/p2pnode` 提供，不是 `cmd/server`。

### Cluster / Health
- `GET /healthz`
- `GET /v1/p2p/raft`
- `POST /v1/p2p/raft/join`
- `POST /v1/p2p/raft/remove`

### State Write
- `POST /v1/p2p/tx`

### State Read
- `GET /v1/p2p/stats`
- `GET /v1/p2p/sessions/{sessionId}`
- `GET /v1/p2p/sessions/{sessionId}/participants`
- `GET /v1/p2p/sessions/{sessionId}/steps/open`
- `GET /v1/p2p/sessions/{sessionId}/events`
- `GET /v1/p2p/steps/{stepId}`
- `GET /v1/p2p/steps/{stepId}/artifacts`

## 文档建议
- 详细字段约束以 `docs/openapi.yaml` 和服务代码为准。
- P2P 的运行与示例参见 `docs/24-p2p-runtime-guide.md`。
