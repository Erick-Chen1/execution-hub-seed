# 兼容性约束（必须保持）

为了保证与现有系统模块兼容，以下约束必须遵守。

## 状态机语义
- Step 状态机与 Action 状态机保持一致。
- Transition 记录必须完整，保证回放。

## 证据链语义
- EvidenceBundle 结构保持一致。
- BundleHash 必须可验证。
- Task 级 Evidence 聚合不能破坏 Action 级 Evidence。

## 审计语义
- 关键操作记录 AuditLog。
- 风险级别分类必须一致。

## Trace ID
- Task、Step、Action、Evidence、Audit 共享 trace_id。

## 人机交接
- 人类确认必须能产生可审计的证据。
- AI 执行输出必须进入 Evidence。
