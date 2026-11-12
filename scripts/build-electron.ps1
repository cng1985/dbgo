# 在 Windows 上直接构建 Electron 桌面发行包。不需要 Git Bash 或 WSL。
$ErrorActionPreference = "Stop"

$currentOnly = $false
foreach ($arg in $args) {
    if ($arg -eq "--current") {
        $currentOnly = $true
    } else {
        throw "未知参数 $arg。只支持 --current"
    }
}

$Version = "0.1.0"
if ($env:DBGO_VERSION) {
    $Version = $env:DBGO_VERSION
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Root = Split-Path -Parent $ScriptDir
Set-Location $Root

if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    throw "未找到 npm"
}
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "未找到 go"
}
if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
    throw "未找到 node"
}

function Invoke-Checked {
    param([scriptblock]$Command)
    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "命令失败，退出码 $LASTEXITCODE"
    }
}

Push-Location (Join-Path $Root "web")
try {
    Invoke-Checked { npm ci }
    Invoke-Checked { npm run build }
} finally {
    Pop-Location
}

$commit = "none"
try {
    $commit = (git -C $Root rev-parse --short HEAD).Trim()
    if (-not $commit) { $commit = "none" }
} catch {
    $commit = "none"
}
$built = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = "-s -w -X github.com/cng1985/dbgo/internal/platform/buildinfo.Version=$Version -X github.com/cng1985/dbgo/internal/platform/buildinfo.Commit=$commit -X github.com/cng1985/dbgo/internal/platform/buildinfo.Time=$built"

$savedGoos = $env:GOOS
$savedGoarch = $env:GOARCH
$env:CGO_ENABLED = "0"
$env:GOTOOLCHAIN = "local"
$hostOs = (& go env GOOS).Trim()
$hostArch = (& go env GOARCH).Trim()

$resources = Join-Path $Root "electron\resources"
if (Test-Path $resources) {
    Remove-Item -LiteralPath $resources -Recurse -Force
}
New-Item -ItemType Directory -Path (Join-Path $resources "configs") | Out-Null
Copy-Item -Path (Join-Path $Root "configs\*") -Destination (Join-Path $resources "configs") -Recurse -Force

function Compile-One {
    param(
        [string]$Folder,
        [string]$Goos,
        [string]$Goarch,
        [string]$Name
    )
    $outDir = Join-Path $resources $Folder
    New-Item -ItemType Directory -Path $outDir -Force | Out-Null
    $out = Join-Path $outDir $Name
    Write-Host "编译 $Folder"
    $env:GOOS = $Goos
    $env:GOARCH = $Goarch
    & go build -trimpath -ldflags $ldflags -o $out ./cmd/server
    if ($LASTEXITCODE -ne 0) {
        throw "编译 $Folder 失败"
    }
}

function Get-HostTarget {
    $folderOs = switch ($hostOs) {
        "windows" { "win" }
        "darwin" { "mac" }
        "linux" { "linux" }
        default { throw "不支持的系统 $hostOs" }
    }
    $name = "dbgo"
    if ($hostOs -eq "windows") { $name = "dbgo.exe" }
    if ($hostOs -eq "windows" -and $hostArch -ne "amd64") {
        throw "Windows 桌面包目前只构建 x64"
    }
    $folderArch = switch ($hostArch) {
        "amd64" { "x64" }
        "arm64" { "arm64" }
        default { throw "不支持的架构 $hostArch" }
    }
    return @{
        Folder = "$folderOs-$folderArch"
        Goos = $hostOs
        Goarch = $hostArch
        Name = $name
    }
}

if ($currentOnly) {
    $hostTarget = Get-HostTarget
    Compile-One $hostTarget.Folder $hostTarget.Goos $hostTarget.Goarch $hostTarget.Name
} else {
    Compile-One "win-x64" "windows" "amd64" "dbgo.exe"
    Compile-One "mac-x64" "darwin" "amd64" "dbgo"
    Compile-One "mac-arm64" "darwin" "arm64" "dbgo"
    Compile-One "linux-x64" "linux" "amd64" "dbgo"
    Compile-One "linux-arm64" "linux" "arm64" "dbgo"
}

$env:GOOS = $savedGoos
$env:GOARCH = $savedGoarch

Push-Location (Join-Path $Root "electron")
try {
    Invoke-Checked { npm ci }
} finally {
    Pop-Location
}

$env:CSC_IDENTITY_AUTO_DISCOVERY = "false"
$onWindows = $env:OS -eq "Windows_NT"

function Stage-Bin {
    param([string]$Folder)
    $bin = Join-Path $resources "bin"
    if (Test-Path $bin) {
        Remove-Item -LiteralPath $bin -Recurse -Force
    }
    New-Item -ItemType Directory -Path $bin | Out-Null
    Copy-Item -Path (Join-Path (Join-Path $resources $Folder) "*") -Destination $bin -Force
}

function Assert-Sidecar {
    param([string]$Folder)
    $bin = switch ($Folder) {
        "win-x64" { Join-Path $Root "electron\dist\win-unpacked\resources\bin\dbgo.exe" }
        "mac-x64" { Join-Path $Root "electron\dist\mac\DBGo.app\Contents\Resources\bin\dbgo" }
        "mac-arm64" { Join-Path $Root "electron\dist\mac-arm64\DBGo.app\Contents\Resources\bin\dbgo" }
        "linux-x64" { Join-Path $Root "electron\dist\linux-unpacked\resources\bin\dbgo" }
        "linux-arm64" { Join-Path $Root "electron\dist\linux-arm64-unpacked\resources\bin\dbgo" }
        default { throw "没有 $Folder 的安装包路径" }
    }
    & node (Join-Path $Root "scripts\electron-package.js") check-binary $bin $Folder
    if ($LASTEXITCODE -ne 0) {
        throw "$Folder 的服务程序架构不对"
    }
}

function Get-PackArgs {
    param([string]$Folder)
    switch ($Folder) {
        "win-x64" { return @("--win", "--x64") }
        "mac-x64" { return @("--mac", "--x64") }
        "mac-arm64" { return @("--mac", "--arm64") }
        "linux-x64" {
            if ($onWindows) { return @("--linux", "tar.gz", "--x64") }
            return @("--linux", "--x64")
        }
        "linux-arm64" {
            if ($onWindows) { return @("--linux", "tar.gz", "--arm64") }
            return @("--linux", "--arm64")
        }
        default { throw "没有 $Folder 的打包参数" }
    }
}

function Pack-One {
    param([string]$Folder)
    Stage-Bin $Folder
    Write-Host "打包 $Folder"
    $packArgs = Get-PackArgs $Folder
    Push-Location (Join-Path $Root "electron")
    try {
        & npx electron-builder @packArgs
        if ($LASTEXITCODE -ne 0) {
            throw "打包 $Folder 失败"
        }
    } finally {
        Pop-Location
    }
    Assert-Sidecar $Folder
}

if ($currentOnly) {
    $hostTarget = Get-HostTarget
    Pack-One $hostTarget.Folder
} else {
    Pack-One "win-x64"
    Pack-One "mac-x64"
    Pack-One "mac-arm64"
    Pack-One "linux-x64"
    Pack-One "linux-arm64"
}

if ($onWindows) {
    & node (Join-Path $Root "scripts\electron-package.js") fix-modes (Join-Path $Root "electron\dist")
    if ($LASTEXITCODE -ne 0) {
        throw "修正可执行权限失败"
    }
}

Write-Host "Electron 发行包在 $(Join-Path $Root 'electron\dist')"
