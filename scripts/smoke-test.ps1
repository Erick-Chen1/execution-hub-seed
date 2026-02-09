$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $root

$logDir = Join-Path $root "tmp\smoke"
$stdout = Join-Path $logDir "p2pnode.out.log"
$stderr = Join-Path $logDir "p2pnode.err.log"
$nodeBin = Join-Path $root "bin\p2pnode-smoke.exe"
$node = $null
$success = $false

function Wait-Health {
  param([string]$Url, [int]$Retries = 60)
  for ($i = 0; $i -lt $Retries; $i++) {
    try {
      $null = Invoke-RestMethod -Method Get -Uri $Url -TimeoutSec 2
      return $true
    } catch {}
    Start-Sleep -Milliseconds 500
  }
  return $false
}

try {
  New-Item -ItemType Directory -Force -Path $logDir | Out-Null
  New-Item -ItemType Directory -Force -Path (Join-Path $root "bin") | Out-Null

  go build -o $nodeBin .\cmd\p2pnode

  $env:P2P_NODE_ID = "smoke-node"
  $env:P2P_RAFT_ADDR = "127.0.0.1:17100"
  $env:P2P_HTTP_ADDR = "127.0.0.1:18100"
  $env:P2P_DATA_DIR = (Join-Path $root "tmp\p2pnode\smoke-node")
  $env:P2P_BOOTSTRAP = "true"

  Remove-Item -Recurse -Force $env:P2P_DATA_DIR -ErrorAction SilentlyContinue
  Remove-Item -Path $stdout, $stderr -ErrorAction SilentlyContinue

  $node = Start-Process -FilePath $nodeBin -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr

  $health = "http://127.0.0.1:18100/healthz"
  if (-not (Wait-Health -Url $health -Retries 80)) {
    throw "P2P node not ready. Check $stdout and $stderr"
  }

  $sessionID = "smoke-session"
  $participantID = "smoke-participant"

  $tx1 = go run .\scripts\p2p-txgen.go --op session-create --session-id $sessionID --actor smoke
  $apply1 = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18100/v1/p2p/tx" -Body $tx1 -ContentType "application/json"

  $tx2 = go run .\scripts\p2p-txgen.go --op participant-join --session-id $sessionID --participant-id $participantID --actor smoke
  $apply2 = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:18100/v1/p2p/tx" -Body $tx2 -ContentType "application/json"

  $session = Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:18100/v1/p2p/sessions/$sessionID"
  $participants = Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:18100/v1/p2p/sessions/$sessionID/participants?limit=20"
  $openSteps = Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:18100/v1/p2p/sessions/$sessionID/steps/open?participant_id=$participantID&limit=20"
  $stats = Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:18100/v1/p2p/stats"

  $summary = [ordered]@{
    tx1_status = $apply1.status
    tx2_status = $apply2.status
    session_id = $session.sessionId
    session_status = $session.status
    participants_count = ($participants.participants | Measure-Object).Count
    open_steps_count = ($openSteps.steps | Measure-Object).Count
    state_stats = $stats
  }
  $summary | ConvertTo-Json -Depth 6

  $success = $true
}
finally {
  if ($node) {
    try { Stop-Process -Id $node.Id -Force } catch {}
  }
  Remove-Item -Recurse -Force (Join-Path $root "tmp\p2pnode\smoke-node") -ErrorAction SilentlyContinue
  if ($success) {
    Remove-Item -Recurse -Force $logDir -ErrorAction SilentlyContinue
  }
}
