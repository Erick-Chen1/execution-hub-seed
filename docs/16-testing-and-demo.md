# 测试与 Demo 指引（P2P Only）

本项目当前验收以 P2P Runtime 为主，不再包含中心化 `cmd/server` 演示链路。

## 建议测试范围
- 节点引导与加入（leader + follower）
- 写事务一致性（`POST /v1/p2p/tx`）
- 状态查询正确性（sessions / participants / open steps / events / stats）
- leader 变更后的写入行为（`NOT_LEADER` 返回与重试策略）

## Demo 演示流程
1. 启动 node-1（bootstrap）
2. 启动 node-2（join）
3. 提交 `SESSION_CREATE` 事务
4. 提交 `PARTICIPANT_JOIN` 事务
5. 查询会话与参与者
6. 查询 open steps 与事件流
7. 展示 `raft` 与 `stats`

## 验收标准
- `healthz` 返回正常
- 写请求在 leader 上 `APPLIED`
- 状态查询结果与已提交事务一致
- 多节点下 raft 状态可见且可管理（join/remove）

## 本地运行提示
- 启动节点: `go run ./cmd/p2pnode`
- 一键启动: `start-win11.cmd`
- 一键停止: `stop-win11.cmd`
- 一键卸载: `uninstall-win11.cmd`

## 可重复冒烟测试
- 运行 `powershell -File scripts/smoke-test.ps1`
- 脚本会自动:
  - 构建并启动 `cmd/p2pnode`
  - 提交最小事务集（session-create / participant-join）
  - 校验查询接口
  - 清理临时数据

## 集成测试说明
- 默认推荐执行 `go test ./...`
- P2P 相关重点: `go test ./internal/p2p/...`
- 旧中心化集成测试（`-tags=integration`）仅作历史兼容参考，不作为当前主验收路径
