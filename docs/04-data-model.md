# 数据模型（更完善版）

## 1. workflow_definitions
- workflow_id (UUID)
- name (string)
- version (int)
- description (string)
- status (ACTIVE/INACTIVE/ARCHIVED)
- definition (jsonb, DAG/步骤模板)
- created_at, created_by
- updated_at, updated_by

## 2. workflow_steps（模板）
- workflow_id (UUID)
- step_key (string, 唯一)
- name (string)
- executor_type (HUMAN/AGENT)
- executor_ref (string, user/group/agent)
- action_type (NOTIFY/WEBHOOK/AGENT_RUN/ESCALATE)
- action_config (jsonb)
- timeout_seconds (int)
- max_retries (int)
- depends_on (text[])
- on_fail (jsonb, escalation policy)

## 3. tasks（任务实例）
- task_id (UUID)
- workflow_id (UUID)
- workflow_version (int)
- title (string)
- status (DRAFT/RUNNING/COMPLETED/FAILED/CANCELLED/BLOCKED)
- context (jsonb)
- trace_id (string)
- created_at, created_by
- updated_at, updated_by

## 4. task_steps（步骤实例）
- step_id (UUID)
- task_id (UUID)
- step_key (string)
- name (string)
- status (复用 Action 状态机)
- executor_type (HUMAN/AGENT)
- executor_ref (string)
- timeout_seconds (int)
- retry_count, max_retries (int)
- depends_on (uuid[])
- action_id (UUID, 对应 actions.action_id)
- evidence (jsonb)
- created_at, dispatched_at, acked_at, resolved_at, failed_at

## 5. executors
- executor_id (string)
- executor_type (HUMAN/AGENT)
- display_name (string)
- capability_tags (text[])
- status (ACTIVE/INACTIVE)
- metadata (jsonb)

## 6. evidence_attachments（可选增强）
- attachment_id (UUID)
- step_id (UUID)
- type (FILE/URL/NOTE)
- ref (string)
- hash (string)
- created_at

## 与现有模块映射
- task_steps -> actions: step_id = action_id，状态机完全复用。
- evidence -> ActionEvidence.Evidence。
- 人机交接 -> notifications + SSE。
- 证据回放 -> EvidenceBundle (ACTION) + Task 聚合。
