# Auth, Users, Approvals

## Users / Agents
- 用户类型: `HUMAN` / `AGENT`
- Agent 必须绑定一个 HUMAN（`owner_user_id`）
- 角色仅三种: `ADMIN` / `OPERATOR` / `VIEWER`
- 密码规则:
  - >= 12 字符
  - 必须包含大小写字母、数字、特殊字符
  - 不得包含用户名
- username 规则:
  - 4-32 字符
  - 以字母开头
  - 仅允许字母、数字、`.`、`_`、`-`
  - 结尾必须是字母或数字

## 人类/Agent 同等待遇（全局 Actor 入口）
- 默认使用当前登录人类用户身份执行操作
- 需要以 Agent 身份执行时，在请求头加入 `X-Actor: agent:<username>`
- 非管理员仅允许使用自己名下 Agent；管理员可使用任意 Agent
- 前端提供全局 Actor 选择器（顶栏），所有请求默认使用该 Actor

## 会话
- Cookie 会话为主，SSE 自动携带
- 也支持 `Authorization: Bearer <session_token>` 用于脚本调用

## Approvals
- 工作流定义可配置 approvals:
  - `task_start`, `task_cancel`, `step_ack`, `step_resolve`, `action_ack`, `action_resolve`
- `applies_to`: `HUMAN` / `AGENT` / `BOTH`
- 审批通过后自动执行原操作
- 审批事件通过 SSE 推送（`event: approval`）
