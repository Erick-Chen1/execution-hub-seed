# 交接清单

新人需要完成
- 阅读 docs/INDEX.md 全部文档
- 理解 Action/Notification/Trust/Audit 语义
- 确认 Workflow/Task/Step/Executor 的最小数据模型
- 实现 Orchestrator MVP
- 打通 Demo 场景

需要明确的技术选择
- 数据库方案
- 事件驱动机制
- 幂等与锁策略

需要保留的兼容点
- Action 状态机
- EvidenceBundle 结构
- Audit 风险等级
- Trace ID 传播

开放问题
- 条件分支表达式是否采用 DSL
- AI 执行器接入方式
- 任务权限模型复杂度
