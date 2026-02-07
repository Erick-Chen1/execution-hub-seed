@echo off
setlocal

set SCRIPT_DIR=%~dp0
powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%scripts\\start-win11.ps1"
if errorlevel 1 (
  echo.
  echo Start script failed. See messages above.
  pause
)
