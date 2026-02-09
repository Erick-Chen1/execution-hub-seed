param(
  [switch]$KeepData,
  [switch]$KeepEnvFile
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$runtimeStateFile = Join-Path $root "tmp\runtime\win11-p2p-processes.json"
Set-Location $root

function Stop-TrackedProcess {
  param(
    [int]$ProcessId,
    [string]$Label
  )

  if ($ProcessId -le 0) { return }
  try {
    $p = Get-Process -Id $ProcessId -ErrorAction SilentlyContinue
    if (-not $p) { return }
    Stop-Process -Id $ProcessId -Force -ErrorAction SilentlyContinue
    if ($Label) {
      Write-Host "Stopped $Label process (PID $ProcessId)"
    } else {
      Write-Host "Stopped process PID $ProcessId"
    }
  } catch {}
}

function Stop-PortProcess {
  param([int]$Port)

  try {
    $pids = Get-NetTCPConnection -LocalPort $Port -ErrorAction SilentlyContinue |
      Select-Object -ExpandProperty OwningProcess -Unique
  } catch {
    $pids = @()
  }

  foreach ($procId in $pids) {
    if (-not $procId) { continue }
    try {
      Stop-Process -Id $procId -Force
      Write-Host "Stopped process on port $Port (PID $procId)"
    } catch {}
  }
}

Write-Host "Stopping local processes..."
if (Test-Path $runtimeStateFile) {
  try {
    $state = Get-Content $runtimeStateFile -Raw | ConvertFrom-Json
    Stop-TrackedProcess -ProcessId ([int]$state.nodePid) -Label "P2P node"
  } catch {}
}
Stop-PortProcess 18080
Stop-PortProcess 18081
Stop-PortProcess 17000
Stop-PortProcess 17001

Write-Host "Cleaning local generated files..."
$pathsToRemove = @(
  (Join-Path $root "tmp\runtime")
)
if (-not $KeepData) {
  $pathsToRemove += (Join-Path $root "tmp\p2pnode")
}
foreach ($path in $pathsToRemove) {
  if (Test-Path $path) {
    Remove-Item -Recurse -Force $path
    Write-Host "Removed $path"
  }
}

if (-not $KeepEnvFile) {
  $envFile = Join-Path $root ".env"
  if (Test-Path $envFile) {
    Remove-Item -Force $envFile
    Write-Host "Removed $envFile"
  }
}

if (Test-Path $runtimeStateFile) {
  Remove-Item -Force $runtimeStateFile -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Host "Uninstall complete."
Write-Host "Hint: run start-win11.cmd to start a fresh P2P node."
