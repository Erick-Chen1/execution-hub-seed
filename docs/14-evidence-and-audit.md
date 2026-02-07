# Evidence 与审计规范

Evidence 最小字段
- step_id
- task_id
- executor_id
- output_summary
- attachments (可选)
- verified_by (可选)
- verified_at (可选)

EvidenceBundle 生成
- 每个 Step 完成后生成 Action Evidence
- Task Evidence 通过聚合 Step Evidence
- BundleHash 必须可验证

审计要求
- 任务创建、启动、取消必须审计
- Step 分发、ACK、RESOLVE、FAIL 必须审计
- 高风险操作标记 risk_level
- 审批请求与审批决策必须审计（含 actor 与 decision）

Trace ID 规则
- Task 创建时生成 trace_id
- Step 与 Action 沿用该 trace_id
- Evidence 与 Audit 使用同一 trace_id
