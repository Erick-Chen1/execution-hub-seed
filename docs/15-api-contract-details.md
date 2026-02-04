# API 合同细化（示例）

## 创建 Workflow
POST /v1/workflows
{
  "name": "report_pipeline",
  "version": 1,
  "description": "...",
  "definition": { ... }
}

响应
{
  "workflow_id": "...",
  "version": 1
}

## 创建 Task
POST /v1/tasks
{
  "workflow_id": "...",
  "title": "研究报告",
  "context": {"topic": "NFC"}
}

响应
{
  "task_id": "...",
  "status": "DRAFT"
}

## 启动 Task
POST /v1/tasks/{taskId}/start

响应
{
  "task_id": "...",
  "status": "RUNNING"
}

## 查询 Steps
GET /v1/tasks/{taskId}/steps

响应
{
  "task_id": "...",
  "steps": [
    {"step_id": "...", "status": "DISPATCHED", "executor_type": "HUMAN"}
  ]
}

错误码约定
- INVALID_PARAM
- NOT_FOUND
- CONFLICT
- INTERNAL_ERROR

分页约定
- limit, cursor
- 响应带 has_more 与 next_cursor
