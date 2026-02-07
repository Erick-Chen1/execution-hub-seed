@echo off
setlocal

set SCRIPT_DIR=%~dp0
powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%scripts\\stop-win11.ps1"
if errorlevel 1 (
  echo.
  echo Stop script failed. See messages above.
  pause
)
