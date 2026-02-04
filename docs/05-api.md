# API 草案（更完善版）

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

## Steps
- GET /v1/tasks/{taskId}/steps
- GET /v1/tasks/{taskId}/steps/{stepId}
- POST /v1/tasks/{taskId}/steps/{stepId}/dispatch
- POST /v1/tasks/{taskId}/steps/{stepId}/ack
- POST /v1/tasks/{taskId}/steps/{stepId}/resolve
- POST /v1/tasks/{taskId}/steps/{stepId}/fail

## Executors
- POST /v1/executors
- GET /v1/executors
- GET /v1/executors/{executorId}
- POST /v1/executors/{executorId}/activate
- POST /v1/executors/{executorId}/deactivate

## Evidence
- GET /v1/tasks/{taskId}/evidence
- GET /v1/tasks/{taskId}/steps/{stepId}/evidence

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
  "actor": "user:alice",
  "comment": "已审核"
}
