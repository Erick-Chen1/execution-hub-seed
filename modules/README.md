# modules 目录说明

此目录包含从原项目复制的关键模块，用于语义对齐和参考实现。

注意事项
- 这些代码在新仓库中不一定可直接编译，可能需要调整 import path 与依赖。
- 复制的目的，是保证状态机、证据链、审计字段与异常语义保持一致。

已复制模块
- `internal/domain/action`
- `internal/domain/audit`
- `internal/domain/notification`
- `internal/domain/rule`
- `internal/domain/trust`
- `internal/application/action`
- `internal/application/audit`
- `internal/application/notification`
- `internal/application/trust`

关键语义必须保持一致
- Action 状态机语义
- Notification 状态机语义
- EvidenceBundle 结构与 BundleHash 校验
- AuditLog 风险等级与签名策略