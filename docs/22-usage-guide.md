# 使用教程

本教程覆盖本地运行、登录、创建工作流、创建任务、推进步骤、审批与审计的最小闭环。更完整的字段说明与约束请参考 `docs/openapi.yaml` 与 `docs/12-workflow-definition-spec.md`。

**快速开始（Win11 一键）**
前置依赖:
- Go 1.24+
- Podman（支持 `podman compose`）
- Node 18+

运行:
```cmd
start-win11.cmd
```

脚本会启动 Postgres、API、Web UI，并打开 `http://localhost:5173`。API 默认 `http://127.0.0.1:8080`。

停止:
```cmd
stop-win11.cmd
```

**手动启动（便于调试）**
启动 Postgres:
```cmd
podman compose -f compose.yaml up -d
```

启动 API:
```cmd
go run ./cmd/server
```

启动 Web:
```cmd
cd web
npm install
npm run dev
```

**初始化管理员与登录（PowerShell 示例）**
密码规则:
- >= 12 字符
- 必须包含大小写字母、数字、特殊字符
- 不得包含用户名

```powershell
$base = "http://127.0.0.1:8080/v1"
$cred = @{ username = "admin"; password = "Strong!Passw0rd" }

# 首次初始化管理员（只允许一次）
Invoke-RestMethod -Method Post -Uri "$base/auth/bootstrap" `
  -Body ($cred | ConvertTo-Json) -ContentType "application/json"

# 登录并保存会话 Cookie
Invoke-RestMethod -Method Post -Uri "$base/auth/login" `
  -Body ($cred | ConvertTo-Json) -ContentType "application/json" `
  -SessionVariable session | Out-Null
```

后续请求使用 `-WebSession $session` 复用 Cookie。也支持 `Authorization: Bearer <session_token>` 用于脚本调用。

**创建用户与 Agent（可选）**
```powershell
# 创建普通用户
$userReq = @{ username = "alice"; password = "S3cure!Passw0rd"; role = "OPERATOR" }
$user = Invoke-RestMethod -Method Post -Uri "$base/users" `
  -Body ($userReq | ConvertTo-Json) -ContentType "application/json" -WebSession $session

# 创建 Agent（绑定 owner_user_id）
$agentReq = @{ username = "writer"; password = "S3cure!Passw0rd"; role = "OPERATOR"; owner_user_id = $user.userId }
Invoke-RestMethod -Method Post -Uri "$base/agents" `
  -Body ($agentReq | ConvertTo-Json) -ContentType "application/json" -WebSession $session
```

需要以 Agent 身份执行时，在请求头加入 `X-Actor: agent:<username>`。非管理员仅允许使用自己名下 Agent。

**创建 Workflow**
```powershell
$wfReq = @{
  name = "report_pipeline"
  description = "report demo"
  steps = @(
    @{
      step_key = "draft"
      name = "AI 草拟"
      executor_type = "AGENT"
      executor_ref = "agent:writer"
      action_type = "AGENT_RUN"
      action_config = @{ prompt = "写一版大纲" }
      timeout_seconds = 900
      max_retries = 2
    },
    @{
      step_key = "review"
      name = "人工复核"
      executor_type = "HUMAN"
      executor_ref = "user:alice"
      action_type = "NOTIFY"
      action_config = @{ title = "请复核"; body = "review draft"; channel = "SSE"; userId = "alice" }
      depends_on = @("draft")
    }
  )
  approvals = @{
    step_resolve = @{ enabled = $true; roles = @("OPERATOR"); applies_to = "HUMAN"; min_approvals = 1 }
  }
}

$wf = Invoke-RestMethod -Method Post -Uri "$base/workflows" `
  -Body ($wfReq | ConvertTo-Json -Depth 8) -ContentType "application/json" -WebSession $session
```

复杂依赖可用 `edges` + `conditions`，示例见 `docs/12-workflow-definition-spec.md`。

**创建 Task 并启动**
```powershell
$taskReq = @{ title = "报告任务"; workflow_id = $wf.workflow_id; context = @{ topic = "NFC" } }
$task = Invoke-RestMethod -Method Post -Uri "$base/tasks" `
  -Body ($taskReq | ConvertTo-Json -Depth 8) -ContentType "application/json" -WebSession $session

Invoke-RestMethod -Method Post -Uri "$base/tasks/$($task.task_id)/start" -WebSession $session | Out-Null
```

**推进 Step（ACK / Resolve / Fail）**
```powershell
$stepsResp = Invoke-RestMethod -Method Get -Uri "$base/tasks/$($task.task_id)/steps" -WebSession $session
$step = $stepsResp.steps | Where-Object { $_.stepKey -eq "review" } | Select-Object -First 1

Invoke-RestMethod -Method Post -Uri "$base/tasks/$($task.task_id)/steps/$($step.stepId)/resolve" `
  -Body (@{ comment = "已复核"; evidence = @{ ok = $true } } | ConvertTo-Json -Depth 6) `
  -ContentType "application/json" -WebSession $session | Out-Null
```

如果触发审批，接口会返回 `202` 与 `approval` 信息。

**审批处理**
```powershell
$approvalId = "<approval-id>"
Invoke-RestMethod -Method Post -Uri "$base/approvals/$approvalId/decide" `
  -Body (@{ decision = "APPROVE"; comment = "同意" } | ConvertTo-Json) `
  -ContentType "application/json" -WebSession $session | Out-Null
```

审批事件会通过 SSE 推送（`GET /v1/notifications/sse`），也可以轮询 `GET /v1/approvals`。

**Evidence / Trust / Audit**
```powershell
# Task Evidence
Invoke-RestMethod -Method Get -Uri "$base/tasks/$($task.task_id)/evidence" -WebSession $session

# Trust 事件与证据包
$eventReq = @{ source_type = "sensor"; source_id = "sensor-1"; event_type = "temp"; schema_version = "1"; payload = @{ value = 42 } }
$eventResp = Invoke-RestMethod -Method Post -Uri "$base/trust/events" `
  -Body ($eventReq | ConvertTo-Json -Depth 6) -ContentType "application/json" -WebSession $session
Invoke-RestMethod -Method Get -Uri "$base/trust/evidence/EVENT/$($eventResp.event_id)" -WebSession $session
Invoke-RestMethod -Method Get -Uri "$base/trust/evidence/TASK/$($task.task_id)" -WebSession $session

# Audit（管理员）
Invoke-RestMethod -Method Get -Uri "$base/admin/audit?limit=20" -WebSession $session
```

**Web 前端使用**
启动后访问 `http://localhost:5173`，页面覆盖 Dashboard / Workflows / Tasks / Executors / Actions / Notifications / Approvals / Trust / Audit / Users。若端口占用，以终端输出为准。
