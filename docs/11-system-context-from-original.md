# 原项目上下文摘要（必要最小集）

原项目名称
- Industrial Data Source (工业数据源系统)

与本项目强相关的能力
- 动作状态机与转移记录
- 通知系统与 SSE 推送
- 证据链与 EvidenceBundle
- 审计日志与风险等级

动作 Action（关键语义）
- 状态: CREATED, DISPATCHING, DISPATCHED, ACKED, RESOLVED, FAILED
- 失败后可重试, 最大次数由 MaxRetries 控制
- 必须记录 action_state_transitions 以支持回放

通知 Notification（关键语义）
- 状态: PENDING, SENT, DELIVERED, FAILED, EXPIRED
- 支持 SSE 与 Webhook
- 失败写入 DLQ 用于后续重试

证据链 EvidenceBundle（关键语义）
- BundleType: EVENT, ACTION, BATCH
- BundleHash 用于完整性校验
- 包含 Events, HashChain, Signatures, Rules, Actions

审计 Audit（关键语义）
- AuditLog 需带 risk_level
- 高风险操作需可查询
- AuditLog 可做签名校验

关键接口已存在于原项目
- Actions: /v1/actions/{actionId}/ack | resolve | transitions | evidence
- Notifications: /v1/notifications/*
- Trust Evidence: /v1/trust/evidence/{bundleType}/{subjectId}
- Audit: /v1/admin/audit/*

为何要保留这些语义
- 新系统要兼容原执行内核, 才能回收进工厂 OS
- 这些语义是“可靠执行”最核心的标准
