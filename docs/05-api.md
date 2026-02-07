# API 草案（更完善版）

认证说明
- 除 /v1/auth/* 之外的接口默认需要登录
- 登录后使用 Cookie 会话（`SESSION_COOKIE_NAME`），SSE 自动携带
- 也支持 `Authorization: Bearer <session_token>` 用于脚本调用
- 需要以 Agent 身份执行时，使用 `X-Actor: agent:<username>`（非管理员仅限自有 Agent）

## Auth
- POST /v1/auth/bootstrap
- POST /v1/auth/login
- POST /v1/auth/logout
- GET /v1/auth/me

## Users / Agents
- POST /v1/users (Admin)
- GET /v1/users (Admin)
- GET /v1/users/{userId} (Admin or self)
- PATCH /v1/users/{userId} (Admin)
- PUT /v1/users/{userId}/password (Admin or self)

- POST /v1/agents
- GET /v1/agents

## Approvals
- GET /v1/approvals
- GET /v1/approvals/{approvalId}
- POST /v1/approvals/{approvalId}/decide

## Workflow
- POST /v1/workflows
- GET /v1/workflows
- GET /v1/workflows/{workflowId}
- GET /v1/workflows/{workflowId}/versions
- POST /v1/workflows/{workflowId}/activate
- POST /v1/workflows/{workflowId}/deactivate

## Tasks
- POST /v1/tasks
- GET /v1/tasks
- GET /v1/tasks/{taskId}
- POST /v1/tasks/{taskId}/start
- POST /v1/tasks/{taskId}/cancel
  - 若触发审批，返回 `202` 与 `approval` 信息

## Steps
- GET /v1/tasks/{taskId}/steps
- GET /v1/tasks/{taskId}/steps/{stepId}
- POST /v1/tasks/{taskId}/steps/{stepId}/dispatch
- POST /v1/tasks/{taskId}/steps/{stepId}/ack
- POST /v1/tasks/{taskId}/steps/{stepId}/resolve
- POST /v1/tasks/{taskId}/steps/{stepId}/fail
  - 若触发审批，返回 `202` 与 `approval` 信息

## Executors
- POST /v1/executors
- GET /v1/executors
- GET /v1/executors/{executorId}
- POST /v1/executors/{executorId}/activate
- POST /v1/executors/{executorId}/deactivate

## Actions (兼容接口)
- POST /v1/actions/{actionId}/ack
- POST /v1/actions/{actionId}/resolve
- GET /v1/actions/{actionId}/transitions
- GET /v1/actions/{actionId}/evidence
  - 若触发审批，返回 `202` 与 `approval` 信息

## Notifications
- GET /v1/notifications
- GET /v1/notifications/{notificationId}
- POST /v1/notifications/{notificationId}/send
- GET /v1/notifications/sse

## Evidence
- GET /v1/tasks/{taskId}/evidence
- GET /v1/tasks/{taskId}/steps/{stepId}/evidence

## Trust
- POST /v1/trust/events
- GET /v1/trust/evidence/{bundleType}/{subjectId}

## Audit
- GET /v1/admin/audit
- GET /v1/admin/audit/{auditId}

## 关键请求样例（示意）
POST /v1/tasks
{
  "title": "研究报告流水线",
  "workflow_id": "...",
  "context": {
    "topic": "NFC 维护策略"
  }
}

POST /v1/tasks/{taskId}/steps/{stepId}/ack
{
  "comment": "已审核"
}
