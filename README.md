# Execution Hub Seed (P2P Only)

Execution Hub 已切换为纯 P2P 协作运行时，仅保留 `cmd/p2pnode` 与 `/v1/p2p/*` 接口。

请从文档索引开始阅读:
- `docs/INDEX.md`

OpenAPI 规范:
- `docs/openapi.yaml`

## 目录结构
- `docs/` 开发目标、需求、架构、模型、接口与计划
- `cmd/` 服务入口
- `internal/` 服务实现
- `web/` 前端项目
- `modules/ids/` 从原仓库复制的参考模块（语义对齐用）

## 本地运行
依赖:
- Go 1.24+

Win 11 一键启动（单节点 P2P）:
```
start-win11.cmd
```

Win 11 一键停止:
```
stop-win11.cmd
```

Win 11 一键卸载（默认会删除 P2P 运行数据与运行时文件）:
```
uninstall-win11.cmd
```

启动 P2P 协作节点（Raft + 状态机）:
```
set P2P_NODE_ID=node-1
set P2P_RAFT_ADDR=127.0.0.1:17000
set P2P_HTTP_ADDR=127.0.0.1:18080
set P2P_BOOTSTRAP=true
go run ./cmd/p2pnode
```

第二个节点加入同一集群:
```
set P2P_NODE_ID=node-2
set P2P_RAFT_ADDR=127.0.0.1:17001
set P2P_HTTP_ADDR=127.0.0.1:18081
set P2P_JOIN_ENDPOINT=http://127.0.0.1:18080
go run ./cmd/p2pnode
```

高级脚本选项（PowerShell）:
```
powershell -File scripts/start-win11.ps1 -SkipOpenBrowser
powershell -File scripts/uninstall-win11.ps1 -KeepData -KeepEnvFile
```

## 测试
- 冒烟测试: `powershell -File scripts/smoke-test.ps1`
- P2P 包测试: `go test ./internal/p2p/...`

## 事务构造
- 使用 `go run ./scripts/p2p-txgen.go --op <op> ...` 生成签名事务 JSON
- 完整参数与 9 个 op 示例见 `docs/24-p2p-runtime-guide.md`

## 默认配置
- P2P 节点: `P2P_NODE_ID`（默认 `node-1`）
- Raft 地址: `P2P_RAFT_ADDR`（默认 `127.0.0.1:17000`）
- HTTP 地址: `P2P_HTTP_ADDR`（默认 `0.0.0.0:18080`）
- 运行数据目录: `P2P_DATA_DIR`（默认 `tmp/p2pnode/<node-id>`）
- 是否引导集群: `P2P_BOOTSTRAP`（默认 `false`）
- 加入入口: `P2P_JOIN_ENDPOINT`（用于非引导节点）
