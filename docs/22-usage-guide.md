# 使用教程

本文给出两条可落地路径：
- 路径 A：使用中心化后端（`cmd/server` + `/v2/collab/*`）
- 路径 B：使用低延迟 P2P Runtime（`cmd/p2pnode` + `/v1/p2p/*`）

## 1. 环境准备
- Go 1.24+
- Node 18+
- Podman（用于本地 Postgres）

## 2. 路径 A：中心化协作 API

### 2.1 启动
```powershell
podman compose -f compose.yaml up -d
go run ./cmd/server
```

### 2.2 初始化管理员并登录
```powershell
$base = "http://127.0.0.1:8080/v1"
$cred = @{ username = "admin"; password = "Strong!Passw0rd" }

Invoke-RestMethod -Method Post -Uri "$base/auth/bootstrap" `
  -Body ($cred | ConvertTo-Json) -ContentType "application/json" | Out-Null

Invoke-RestMethod -Method Post -Uri "$base/auth/login" `
  -Body ($cred | ConvertTo-Json) -ContentType "application/json" `
  -SessionVariable session | Out-Null
```

### 2.3 创建协作会话
```powershell
$create = @{
  workflow_id = "<workflow-uuid>"
  title = "demo-collab"
  context = @{ topic = "compiler" }
}
$sessionResp = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:8080/v2/collab/sessions" `
  -Body ($create | ConvertTo-Json -Depth 8) -ContentType "application/json" -WebSession $session

$sessionId = $sessionResp.sessionId
```

### 2.4 加入会话并查询开放步骤
```powershell
$join = @{ type = "HUMAN"; ref = "user:admin"; capabilities = @("draft","review") }
$participant = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:8080/v2/collab/sessions/$sessionId/join" `
  -Body ($join | ConvertTo-Json -Depth 6) -ContentType "application/json" -WebSession $session

$pid = $participant.participantId

Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:8080/v2/collab/sessions/$sessionId/steps/open?participant_id=$pid" -WebSession $session
```

### 2.5 认领步骤并提交产物
```powershell
$stepId = "<open-step-id>"

Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:8080/v2/collab/steps/$stepId/claim" `
  -Body (@{ participant_id = $pid; lease_seconds = 600 } | ConvertTo-Json) `
  -ContentType "application/json" -WebSession $session

Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:8080/v2/collab/steps/$stepId/artifacts" `
  -Body (@{ participant_id = $pid; kind = "draft"; content = @{ text = "hello" } } | ConvertTo-Json -Depth 8) `
  -ContentType "application/json" -WebSession $session
```

### 2.6 发起决策、投票、解决步骤
```powershell
$decision = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:8080/v2/collab/steps/$stepId/decisions" `
  -Body (@{} | ConvertTo-Json) -ContentType "application/json" -WebSession $session

$decisionId = $decision.decisionId

Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:8080/v2/collab/decisions/$decisionId/votes" `
  -Body (@{ participant_id = $pid; choice = "APPROVE" } | ConvertTo-Json) `
  -ContentType "application/json" -WebSession $session

Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:8080/v2/collab/steps/$stepId/resolve" `
  -Body (@{ participant_id = $pid } | ConvertTo-Json) `
  -ContentType "application/json" -WebSession $session
```

## 3. 路径 B：P2P Runtime

详细说明见 `docs/24-p2p-runtime-guide.md`，这里给最短流程。

### 3.1 启动节点 1（引导）
```powershell
$env:P2P_NODE_ID = "node-1"
$env:P2P_RAFT_ADDR = "127.0.0.1:17000"
$env:P2P_HTTP_ADDR = "127.0.0.1:18080"
$env:P2P_BOOTSTRAP = "true"
go run ./cmd/p2pnode
```

### 3.2 启动节点 2（加入）
```powershell
$env:P2P_NODE_ID = "node-2"
$env:P2P_RAFT_ADDR = "127.0.0.1:17001"
$env:P2P_HTTP_ADDR = "127.0.0.1:18081"
$env:P2P_JOIN_ENDPOINT = "http://127.0.0.1:18080"
go run ./cmd/p2pnode
```

### 3.3 检查集群
```powershell
Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:18080/healthz"
Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:18080/v1/p2p/raft"
```

### 3.4 提交事务
P2P 写入统一走 `POST /v1/p2p/tx`，请求体是已签名 `protocol.Tx`。字段和签名规则参考：
- `internal/p2p/protocol/types.go`
- `internal/p2p/protocol/types_test.go`

### 3.5 查询状态
```powershell
Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:18080/v1/p2p/stats"
Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:18080/v1/p2p/sessions/<session-id>"
Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:18080/v1/p2p/sessions/<session-id>/events"
```

## 4. 常见问题
- 收到 `NOT_LEADER`：写请求打到了 follower，请改投 leader 地址。
- 看到步骤不可认领：通常是依赖未完成、能力不匹配或已有有效租约。
- 会话不自动完成：需确保会话内所有步骤都进入 `RESOLVED`。
