# 复用模块与映射

## 已复制模块（modules/ids/internal/...）
- domain/action
- domain/audit
- domain/notification
- domain/rule
- domain/trust
- application/action
- application/audit
- application/notification
- application/trust

## 复用方式
- Step 状态机直接复用 Action（含转移记录与重试逻辑）。
- 人机交接复用 Notification + SSE。
- 证据链复用 Trust EvidenceBundle。
- 审计复用 Audit Service 与 RiskLevel。

## 依赖提示
- application/action 依赖 domain/rule.Repository。
- application/notification 依赖 domain/action + domain/rule。
- application/trust 依赖 trust.Repository + trust.KeyStore。
- application/audit 依赖 audit.Repository。

## 在新仓库中的落地建议
- 将 modules/ids 中的包移入新仓库的 domain/application 层。
- 重命名模块路径（替换 import path）。
- 保持 Action/Trust/Audit 结构与字段语义一致，确保兼容。
