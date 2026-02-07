# 开发前约定与模块扩展点清单

本文件用于在开发前统一关键约定与扩展点，确保兼容性与可维护性，避免返工。

## 适用范围
- 本仓库中的 `modules/ids/internal/*` 复用模块
- Execution Hub 的核心模型与编排逻辑（workflow/task/step/executor）

## 全局约定

### 标识与时间
- 所有对外实体使用 `uuid`（actionId、notificationId、auditId、bundleId 等）
- 数据库内部可使用 `int64` 作为主键
- 全系统时间戳统一为 UTC
- `trace_id` 贯穿 Task/Step/Action/Notification/Evidence/Audit

### 状态机与可追溯
- 状态迁移必须通过领域对象方法完成，禁止直接写状态字段
- Action/Step 每次迁移必须记录 `StateTransition`
- 终态定义需一致，避免“可重试但被判定终态”

### 证据链与审计
- Evidence 必须可聚合为 EvidenceBundle，且 `BundleHash` 可验证
- 审计日志包含 `risk_level` 与关键上下文（actor/action/entity）
- 高风险操作必须标记并可检索

### 配置与兼容
- 所有 `config` / `actionConfig` 必须是合法 JSON
- 兼容性优先级最高，不得破坏 Action/Trust/Audit 语义

### 错误与重试
- 不可重试错误需显式区分（如 4xx webhook）
- 重试逻辑以 `MaxRetries` / `RetryCount` 为准，不能绕过领域约束

### 日志与可观测
- 关键状态变化必须记录结构化日志（至少含 `trace_id` 与实体 ID）
- 异步失败不得吞掉，需落盘或进入 DLQ

## 模块结构与扩展点

### Domain.Action
- 关键结构: Action, StateTransition, Repository
- 状态机: CREATED → DISPATCHING → DISPATCHED → ACKED/RESOLVED/FAILED
- 扩展点: 新 ActionType 需补齐解析与执行路径；Dedupe/TTL/Cooldown 需落地

### Domain.Notification
- 关键结构: Notification, DeliveryAttempt, SSEClient, Repository/SSEHub
- 状态机: PENDING → SENT → DELIVERED/FAILED；EXPIRED 为终态
- 扩展点: 新 Channel 需补齐发送实现与错误分类

### Domain.Audit
- 关键结构: AuditLog, QueryFilter(Cursor), Repository
- 约束: 风险分级入口统一在 `DetermineRiskLevel`
- 扩展点: 新 EntityType/Action 需同步风控策略

### Domain.Rule
- 关键结构: Rule, Evaluation
- 约束: Validate 确保 RuleType/ActionType 与配置合法
- 扩展点: 新 RuleType/ActionType 需补齐配置与解析映射

### Domain.Trust
- 关键结构: HashChainEntry, BatchSignature, EvidenceBundle, KeyStore
- 约束: HashChain 连续、EvidenceBundle Finalize 必做
- 扩展点: 新签名算法需扩展 KeyStore 与校验流程

### Application.Action
- 关键方法: CreateFromEvaluation / Dispatch / Ack / Resolve / Fail / GetEvidence
- 约束: 状态迁移持久化 + 记录 Transition
- 扩展点: 新 ActionType 需补齐创建与调度逻辑

### Application.Notification
- 关键方法: CreateFromAction / Send / Retry / Expire
- 约束: 发送前置 SENT 状态；Webhook 4xx 不可重试
- 扩展点: 批量发送、速率限制、指数退避

### Application.Audit
- 关键方法: Log / Query / GetByID / VerifyIntegrity
- 约束: Query limit 上限 200，默认 50

### Application.Trust
- 关键方法: AddToHashChain / RegisterBatchSignature / VerifyHashChain / GenerateEvidenceBundle
- 约束: Batch 验证成功后提升 TrustLevel

## 集成约束（Execution Hub）
- Step 与 Action 必须一一映射（StepID = ActionID）
- Step 状态机语义与 Action 完全一致
- Task/Step Evidence 可聚合为 Trust EvidenceBundle
- 人机交互基于 Notification/SSE，ACK/RESOLVE 必须审计
- Workflow 版本冻结到 Task 实例
- Orchestrator 必须幂等，避免重复创建 Action

## 兼容接口保留
- Actions: `/v1/actions/{actionId}/ack | resolve | transitions | evidence`
- Notifications: `/v1/notifications/*`
- Trust Evidence: `/v1/trust/evidence/{bundleType}/{subjectId}`
- Audit: `/v1/admin/audit/*`

## 开发计划（阶段约束）
- Phase 1: Workflow/Task/Step/Executor 模型 + Repository + TraceID
- Phase 2: Orchestrator Step→Action 映射、超时与重试
- Phase 3: Notification/SSE + Trust EvidenceBundle 闭环
- Phase 4: API 层 + Demo 全链路演示

## 完整检查清单
- 关键实体均含 `trace_id` 并跨服务传播
- Action/Notification 迁移通过领域方法并记录 Transition
- EvidenceBundle 可生成且 `BundleHash` 可验证
- Audit 能按实体/trace 查询高风险操作
- 新类型（Action/Channel/Rule/BundleType）具备解析与校验逻辑
- Step/Action 语义一致且可回放迁移记录
- Orchestrator 幂等

## 检查记录
- 2026-02-07: 重建文档与校准扩展点，完成完整检查
