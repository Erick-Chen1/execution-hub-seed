# 总体架构

核心组件
- Workflow Registry: 流程定义与版本管理。
- Task Engine: 任务实例化、步骤推进、状态更新。
- Orchestrator: 依赖检查、超时、重试、升级策略。
- Executor Registry: 统一执行者模型（人/AI）。
- Evidence Service: 证据收集与 EvidenceBundle 生成。
- Audit Service: 操作审计与风险标记。
- Notification/SSE: 人机交接通道。

数据流概览
1. Workflow 定义 -> Task 实例化。
2. Orchestrator 选取可执行步骤 -> 创建 Action。
3. Action 分发 -> AI 执行或人类确认。
4. 执行结果写入 Evidence -> 生成 EvidenceBundle。
5. 审计记录与回放。

技术复用
- Step 状态机复用 Action 状态机。
- 证据链复用 Trust 模块 EvidenceBundle。
- 人机交接复用 Notification + SSE。

建议的最小技术分层
- domain: workflow/task/step/executor 领域模型
- application: orchestration, evidence, audit
- infrastructure: persistence, notification, sse
- api: workflow/task/step/evidence endpoints
