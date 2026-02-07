$ErrorActionPreference = "Stop"

$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $root

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
    } catch {
      Write-Host "Failed to stop PID $procId on port $Port"
    }
  }
}

Stop-PortProcess 5173
Stop-PortProcess 8080

if (Get-Command podman -ErrorAction SilentlyContinue) {
  try {
    podman compose -f compose.yaml down | Out-Null
    Write-Host "Stopped Postgres container."
  } catch {
    Write-Host "Failed to stop Podman compose. Check Podman status."
  }
} else {
  Write-Host "Podman not found. Skipped container shutdown."
}

Write-Host "Done."
