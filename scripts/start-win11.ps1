$ErrorActionPreference = "Stop"

$root = Resolve-Path (Join-Path $PSScriptRoot "..")
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

Assert-Command "podman" "Install Podman Desktop (podman compose required)."
Assert-Command "go" "Install Go 1.24+."
Assert-Command "node" "Install Node 18+ (npm included)."
Assert-Command "npm" "Install Node 18+ (npm included)."

$envFile = Join-Path $root ".env"
$envExample = Join-Path $root ".env.example"
if (-not (Test-Path $envFile) -and (Test-Path $envExample)) {
  Copy-Item $envExample $envFile
  Write-Host "Created .env from .env.example"
}

function Load-DotEnv {
  param([string]$Path)
  if (-not (Test-Path $Path)) { return }
  foreach ($line in Get-Content $Path) {
    $trimmed = $line.Trim()
    if (-not $trimmed) { continue }
    if ($trimmed.StartsWith("#")) { continue }
    $pair = $trimmed.Split("=", 2)
    if ($pair.Count -ne 2) { continue }
    $key = $pair[0].Trim()
    $value = $pair[1].Trim()
    if ($key) { Set-Item -Path ("env:{0}" -f $key) -Value $value }
  }
}

Load-DotEnv $envFile

$podmanOk = $true
try { podman info | Out-Null } catch { $podmanOk = $false }
if (-not $podmanOk) {
  Write-Host "Podman not running. Starting podman machine..."
  try { podman machine start | Out-Null } catch {}
  $podmanOk = $true
  try { podman info | Out-Null } catch { $podmanOk = $false }
  if (-not $podmanOk) {
    Write-Host "Podman machine not initialized. Initializing..."
    try { podman machine init | Out-Null } catch {}
    try { podman machine start | Out-Null } catch {}
    $podmanOk = $true
    try { podman info | Out-Null } catch { $podmanOk = $false }
    if (-not $podmanOk) {
      Write-Host "Podman is still unavailable. Please run: podman machine init"
      exit 1
    }
  }
}

Write-Host "Starting Postgres container..."
podman compose -f compose.yaml up -d | Out-Null
if ($LASTEXITCODE -ne 0) {
  Write-Host "podman compose failed. Check your Podman setup and compose.yaml."
  exit 1
}

$pgUser = if ($env:POSTGRES_USER) { $env:POSTGRES_USER } else { "exec_hub" }
$pgDb = if ($env:POSTGRES_DB) { $env:POSTGRES_DB } else { "exec_hub" }
$ready = $false
for ($i = 0; $i -lt 30; $i++) {
  podman exec execution-hub-postgres pg_isready -U $pgUser -d $pgDb | Out-Null
  if ($LASTEXITCODE -eq 0) { $ready = $true; break }
  Start-Sleep -Seconds 1
}
if (-not $ready) {
  Write-Host "Warning: Postgres not ready yet. The API may fail to start immediately."
}

Write-Host "Starting API server..."
$serverArgs = @("-NoExit", "-NoProfile", "-Command", "go run ./cmd/server")
Start-Process -FilePath "powershell" -WorkingDirectory $root -ArgumentList $serverArgs | Out-Null

Write-Host "Starting web UI..."
$webDir = Join-Path $root "web"
$webCmd = "if (-not (Test-Path 'node_modules')) { npm install }; npm run dev"
$webArgs = @("-NoExit", "-NoProfile", "-Command", $webCmd)
Start-Process -FilePath "powershell" -WorkingDirectory $webDir -ArgumentList $webArgs | Out-Null

Start-Sleep -Seconds 2
$webUrl = "http://localhost:5173"
Start-Process $webUrl | Out-Null

Write-Host ""
Write-Host "Done."
Write-Host "API: http://127.0.0.1:8080"
Write-Host "Web: $webUrl"
