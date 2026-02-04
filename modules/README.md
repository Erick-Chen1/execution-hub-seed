# modules 目录说明

此目录包含从原项目复制的关键模块，用作参考实现与语义对齐。

注意事项
- 这些代码在新仓库中不一定能直接编译，因为 import path 需要调整。
- 复制的目的，是让新人能精确理解状态机、证据链、审计字段与失败语义。

已复制模块
- internal/domain/action
- internal/domain/audit
- internal/domain/notification
- internal/domain/rule
- internal/domain/trust
- internal/application/action
- internal/application/audit
- internal/application/notification
- internal/application/trust

关键语义必须保持一致
- Action 状态机语义
- Notification 状态机语义
- EvidenceBundle 结构与 BundleHash 校验
- AuditLog 风险等级与签名策略
