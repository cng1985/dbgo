@echo off
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0build-electron.ps1" %*
exit /b %ERRORLEVEL%
