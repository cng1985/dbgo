@echo off
setlocal
where bash >nul 2>&1
if errorlevel 1 (
  echo 需要 Git Bash 或 WSL 来运行 scripts\build-electron.sh
  exit /b 1
)
bash "%~dp0build-electron.sh" %*
exit /b %ERRORLEVEL%
