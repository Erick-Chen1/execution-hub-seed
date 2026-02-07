$ErrorActionPreference = "Stop"

$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $root

$env:POSTGRES_USER = "exec_hub"
$env:POSTGRES_PASSWORD = "exec_hub_pass"
$env:POSTGRES_DB = "exec_hub_smoke"
$env:POSTGRES_PORT = "55432"
$env:POSTGRES_HOST = "127.0.0.1"
$env:DATABASE_SSLMODE = "disable"
$env:SERVER_ADDR = "127.0.0.1:18080"
$env:AUDIT_SIGNING_KEY = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

$base = "http://127.0.0.1:18080/v1"
$proc = $null
$success = $false
$logDir = Join-Path $root "tmp\smoke"
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
$stdout = Join-Path $logDir "server.out.log"
$stderr = Join-Path $logDir "server.err.log"

try {
  podman compose down -v | Out-Null
  podman compose up -d | Out-Null

  $ready = $false
  for ($i = 0; $i -lt 30; $i++) {
    podman exec execution-hub-postgres pg_isready -U $env:POSTGRES_USER -d $env:POSTGRES_DB | Out-Null
    if ($LASTEXITCODE -eq 0) { $ready = $true; break }
    Start-Sleep -Seconds 1
  }
  if (-not $ready) { throw "Postgres not ready" }

  New-Item -ItemType Directory -Force -Path .\bin | Out-Null
  go build -o .\bin\execution-hub.exe .\cmd\server

  Remove-Item -Path $stdout, $stderr -ErrorAction SilentlyContinue
  $proc = Start-Process -FilePath .\bin\execution-hub.exe -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr

  $serverReady = $false
  $bootstrap = @{ username = "alice"; password = "S3cure!Passw0rd" }
  for ($i = 0; $i -lt 30; $i++) {
    try {
      Invoke-RestMethod -Method Post -Uri "$base/auth/bootstrap" -Body ($bootstrap | ConvertTo-Json) -ContentType "application/json" | Out-Null
    } catch {}
    try {
      $login = Invoke-RestMethod -Method Post -Uri "$base/auth/login" -Body ($bootstrap | ConvertTo-Json) -ContentType "application/json" -SessionVariable session
      if ($session) { $serverReady = $true; break }
    } catch {}
    Start-Sleep -Seconds 1
  }
  if (-not $serverReady) { throw "Server not ready or login failed. Check $stdout and $stderr." }

  $wfReq = @{
    name = "smoke_workflow"
    steps = @(
      @{
        step_key = "s1"
        name = "Step1"
        executor_type = "HUMAN"
        executor_ref = "user:alice"
        action_type = "NOTIFY"
        action_config = @{ title = "Hello"; body = "Step1"; channel = "SSE"; userId = "alice" }
        timeout_seconds = 0
        max_retries = 1
      },
      @{
        step_key = "s2"
        name = "Step2"
        executor_type = "HUMAN"
        executor_ref = "user:alice"
        action_type = "NOTIFY"
        action_config = @{ title = "Hello2"; body = "Step2"; channel = "SSE"; userId = "alice" }
        timeout_seconds = 0
        max_retries = 1
        depends_on = @("s1")
      }
    )
  }
  $wf = Invoke-RestMethod -Method Post -Uri "$base/workflows" -Body ($wfReq | ConvertTo-Json -Depth 8) -ContentType "application/json" -WebSession $session

  $taskReq = @{ title = "smoke task"; workflow_id = $wf.workflow_id; context = @{ flag = $true } }
  $task = Invoke-RestMethod -Method Post -Uri "$base/tasks" -Body ($taskReq | ConvertTo-Json -Depth 8) -ContentType "application/json" -WebSession $session

  Invoke-RestMethod -Method Post -Uri "$base/tasks/$($task.task_id)/start" -WebSession $session | Out-Null

  $stepsResp = Invoke-RestMethod -Method Get -Uri "$base/tasks/$($task.task_id)/steps" -WebSession $session
  $step1 = $stepsResp.steps | Where-Object { $_.stepKey -eq "s1" } | Select-Object -First 1
  if (-not $step1) { throw "step1 not found" }

  Invoke-RestMethod -Method Post -Uri "$base/tasks/$($task.task_id)/steps/$($step1.stepId)/resolve" -Body (@{ evidence = @{ ok = $true } } | ConvertTo-Json -Depth 6) -ContentType "application/json" -WebSession $session | Out-Null

  $stepsResp = Invoke-RestMethod -Method Get -Uri "$base/tasks/$($task.task_id)/steps" -WebSession $session
  $step2 = $stepsResp.steps | Where-Object { $_.stepKey -eq "s2" } | Select-Object -First 1
  if (-not $step2) { throw "step2 not found" }

  Invoke-RestMethod -Method Post -Uri "$base/tasks/$($task.task_id)/steps/$($step2.stepId)/resolve" -Body (@{ evidence = @{ ok = $true } } | ConvertTo-Json -Depth 6) -ContentType "application/json" -WebSession $session | Out-Null

  $stepsResp = Invoke-RestMethod -Method Get -Uri "$base/tasks/$($task.task_id)/steps" -WebSession $session
  $taskFinal = Invoke-RestMethod -Method Get -Uri "$base/tasks/$($task.task_id)" -WebSession $session
  $notifications = Invoke-RestMethod -Method Get -Uri "$base/notifications?limit=10" -WebSession $session
  $taskEvidence = Invoke-RestMethod -Method Get -Uri "$base/tasks/$($task.task_id)/evidence" -WebSession $session

  $eventReq = @{ source_type = "sensor"; source_id = "sensor-1"; event_type = "temp"; schema_version = "1"; payload = @{ value = 42 } }
  $eventResp = Invoke-RestMethod -Method Post -Uri "$base/trust/events" -Body ($eventReq | ConvertTo-Json -Depth 6) -ContentType "application/json" -WebSession $session
  $eventBundle = Invoke-RestMethod -Method Get -Uri "$base/trust/evidence/EVENT/$($eventResp.event_id)" -WebSession $session

  $taskBundle = Invoke-RestMethod -Method Get -Uri "$base/trust/evidence/TASK/$($task.task_id)" -WebSession $session
  $audit = Invoke-RestMethod -Method Get -Uri "$base/admin/audit?limit=5" -WebSession $session

  $summary = @{
    workflow_id = $wf.workflow_id
    task_id = $task.task_id
    task_status = $taskFinal.status
    steps = ($stepsResp.steps | Select-Object stepKey, status)
    notifications_count = ($notifications.notifications | Measure-Object).Count
    audit_count = ($audit.logs | Measure-Object).Count
    event_id = $eventResp.event_id
    task_bundle_type = $taskBundle.bundleType
  }
  $summary | ConvertTo-Json -Depth 6
  $success = $true
}
finally {
  if ($proc) { try { Stop-Process -Id $proc.Id -Force } catch {} }
  try { podman compose down -v | Out-Null } catch {}
  if ($success) { Remove-Item -Recurse -Force $logDir -ErrorAction SilentlyContinue }
}
