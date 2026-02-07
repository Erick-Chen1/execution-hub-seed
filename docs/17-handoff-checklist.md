# 交接清单（2026-02-07）

## 已完成
- 已完成 V2 协作主链路：session / participant / claim / handoff / artifact / decision / vote / resolve。
- 已提供协作事件流与查询接口：`/v2/collab/*`。
- 已新增低延迟 P2P Runtime MVP：
  - 启动入口：`cmd/p2pnode/main.go`
  - 协议：`internal/p2p/protocol`
  - 状态机：`internal/p2p/state`
  - 共识层：`internal/p2p/consensus`（Raft）
  - API：`internal/p2p/api`
- 已补充 P2P 文档：`docs/24-p2p-runtime-guide.md`。
- 已完成基础测试并通过：`go test ./...`。

## 新同学接手前先做
- 通读 `docs/INDEX.md`，重点看：`docs/23-leaderless-multiagent-design.md`、`docs/24-p2p-runtime-guide.md`。
- 本地拉起 2 节点 P2P 集群并验证 join + tx apply。
- 走一遍最小闭环：创建会话 -> 加入参与者 -> 认领步骤 -> 提交产物 -> 发起决策 -> 投票 -> 解决步骤。

## 待补强（下一阶段）
- P2P 鉴权与签名密钥分发（当前仅验签，不负责密钥托管）。
- 节点发现与网络层增强（gossip、NAT 穿透、跨网段部署）。
- P2P API 的 OpenAPI 描述和 SDK 生成。
- 端到端自动化测试（多进程多节点场景）。
- 前端直接对接 P2P Runtime 的观测与操作面板。

## 风险点
- 单机多节点演示稳定，跨主机网络环境尚未系统验证。
- 当前使用 Raft 强一致，吞吐与延迟随节点数增加会下降。
- 若直接在公网开放，需要补齐 TLS、认证、限流与审计策略。

## 验收基线
- 2 节点下读写一致，follower 重启可自动追平。
- 任意写请求打到 follower 时返回可用 leader 信息。
- 关键状态机路径有单元测试覆盖并保持通过。
