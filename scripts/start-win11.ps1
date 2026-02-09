param(
  [switch]$SkipOpenBrowser
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$runtimeDir = Join-Path $root "tmp\runtime"
$runtimeStateFile = Join-Path $runtimeDir "win11-p2p-processes.json"
Set-Location $root

function Assert-Command {
  param(
    [string]$Name,
    [string]$Hint
  )

  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    Write-Host "Missing required command: $Name"
    if ($Hint) { Write-Host $Hint }
    exit 1
  }
}

function Stop-TrackedProcess {
  param([int]$ProcessId)
  if ($ProcessId -le 0) { return }
  try {
    $p = Get-Process -Id $ProcessId -ErrorAction SilentlyContinue
    if ($p) {
      Stop-Process -Id $ProcessId -Force -ErrorAction SilentlyContinue
      Write-Host "Stopped stale tracked process PID $ProcessId"
    }
  } catch {}
}

if (-not (Test-Path $runtimeDir)) {
  New-Item -ItemType Directory -Path $runtimeDir -Force | Out-Null
}

if (Test-Path $runtimeStateFile) {
  try {
    $state = Get-Content $runtimeStateFile -Raw | ConvertFrom-Json
    Stop-TrackedProcess -ProcessId ([int]$state.nodePid)
  } catch {}
  Remove-Item $runtimeStateFile -Force -ErrorAction SilentlyContinue
}

Assert-Command "go" "Install Go 1.24+."

$nodeID = if ($env:P2P_NODE_ID) { $env:P2P_NODE_ID } else { "node-1" }
$raftAddr = if ($env:P2P_RAFT_ADDR) { $env:P2P_RAFT_ADDR } else { "127.0.0.1:17000" }
$httpAddr = if ($env:P2P_HTTP_ADDR) { $env:P2P_HTTP_ADDR } else { "127.0.0.1:18080" }
$dataDir = if ($env:P2P_DATA_DIR) { $env:P2P_DATA_DIR } else { "tmp/p2pnode/$nodeID" }
$bootstrap = if ($env:P2P_BOOTSTRAP) { $env:P2P_BOOTSTRAP } else { "true" }
$joinEndpoint = if ($env:P2P_JOIN_ENDPOINT) { $env:P2P_JOIN_ENDPOINT } else { "" }

Write-Host "Starting P2P node..."
$startCmd = @(
  "`$env:P2P_NODE_ID='$nodeID'",
  "`$env:P2P_RAFT_ADDR='$raftAddr'",
  "`$env:P2P_HTTP_ADDR='$httpAddr'",
  "`$env:P2P_DATA_DIR='$dataDir'",
  "`$env:P2P_BOOTSTRAP='$bootstrap'"
)
if ($joinEndpoint -ne "") {
  $startCmd += "`$env:P2P_JOIN_ENDPOINT='$joinEndpoint'"
}
$startCmd += "go run ./cmd/p2pnode"
$nodeArgs = @("-NoExit", "-NoProfile", "-Command", ($startCmd -join "; "))
$nodeProc = Start-Process -FilePath "powershell" -WorkingDirectory $root -ArgumentList $nodeArgs -PassThru

$stateObj = [pscustomobject]@{
  nodePid   = $nodeProc.Id
  nodeId    = $nodeID
  raftAddr  = $raftAddr
  httpAddr  = $httpAddr
  startedAt = (Get-Date).ToString("o")
}
$stateObj | ConvertTo-Json | Set-Content -Encoding UTF8 $runtimeStateFile

$healthURL = "http://$httpAddr/healthz"
$ready = $false
for ($i = 0; $i -lt 40; $i++) {
  try {
    $null = Invoke-RestMethod -Method Get -Uri $healthURL -TimeoutSec 2
    $ready = $true
    break
  } catch {}
  Start-Sleep -Milliseconds 500
}

Write-Host ""
if ($ready) {
  Write-Host "Done."
} else {
  Write-Host "Started, but health check is not ready yet."
}
Write-Host "P2P API: http://$httpAddr (PID $($nodeProc.Id))"
Write-Host "Health: $healthURL"
Write-Host "Runtime state: $runtimeStateFile"

if (-not $SkipOpenBrowser) {
  Start-Process $healthURL | Out-Null
}
