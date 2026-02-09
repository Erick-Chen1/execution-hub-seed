# 使用教程（P2P Only）

本项目已移除中心化运行路径，仅保留 `cmd/p2pnode` 与 `/v1/p2p/*`。

## 1. 环境准备
- Go 1.24+

## 2. 一键启动（Windows 11）
```powershell
start-win11.cmd
```

停止:
```powershell
stop-win11.cmd
```

卸载（删除本地运行数据）:
```powershell
uninstall-win11.cmd
```

## 3. 手动启动单节点
```powershell
$env:P2P_NODE_ID = "node-1"
$env:P2P_RAFT_ADDR = "127.0.0.1:17000"
$env:P2P_HTTP_ADDR = "127.0.0.1:18080"
$env:P2P_BOOTSTRAP = "true"
go run ./cmd/p2pnode
```

## 4. 手动启动第二节点加入集群
```powershell
$env:P2P_NODE_ID = "node-2"
$env:P2P_RAFT_ADDR = "127.0.0.1:17001"
$env:P2P_HTTP_ADDR = "127.0.0.1:18081"
$env:P2P_JOIN_ENDPOINT = "http://127.0.0.1:18080"
$env:P2P_BOOTSTRAP = "false"
go run ./cmd/p2pnode
```

## 5. 基本检查
```powershell
Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:18080/healthz"
Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:18080/v1/p2p/raft"
Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:18080/v1/p2p/stats"
```

## 6. 写入事务
P2P 写入统一使用 `POST /v1/p2p/tx`，请求体必须是已签名 `protocol.Tx`。

参考:
- `internal/p2p/protocol/types.go`
- `scripts/p2p-txgen.go`
- `scripts/smoke-test.ps1`

## 7. 常见问题
- 返回 `NOT_LEADER`: 写请求发到了 follower，请改发到 leader。
- 查询不到 session: 先确认对应 `SESSION_CREATE` 事务已 `APPLIED`。
- open steps 为空: 检查步骤依赖、能力匹配和 claim 状态。
