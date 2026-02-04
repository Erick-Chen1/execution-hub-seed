# Workflow 定义规范（建议）

目标
- 以 JSON 定义流程, 支持并行与条件分支
- Workflow 定义是版本化的, Task 实例化时冻结版本

字段约定
- workflow_id, version, name, description
- steps: 步骤定义列表
- edges: 依赖关系
- conditions: 可选条件表达式

示例
{
  "workflow_id": "...",
  "version": 1,
  "name": "report_pipeline",
  "steps": [
    {
      "step_key": "draft",
      "name": "AI 草拟大纲",
      "executor_type": "AGENT",
      "executor_ref": "agent:writer",
      "action_type": "AGENT_RUN",
      "action_config": {"prompt": "..."},
      "timeout_seconds": 900,
      "max_retries": 2
    },
    {
      "step_key": "review",
      "name": "人工复核",
      "executor_type": "HUMAN",
      "executor_ref": "user:alice",
      "action_type": "NOTIFY",
      "action_config": {"title": "请复核", "body": "..."},
      "timeout_seconds": 3600,
      "max_retries": 1
    }
  ],
  "edges": [
    {"from": "draft", "to": "review"}
  ]
}

条件与分支
- 可在 edge 上加 condition
- condition 由 Orchestrator 解析, 最小版本可只支持 true/false

升级策略
- step 上可选 on_fail
- 示例: {"type": "ESCALATE", "target": "group:ops"}
