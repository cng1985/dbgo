# 构建前端一次，再交叉编译各平台二进制并打包。任何一步失败都返回非零。
param(
    [string]$Version = $(if ($env:DBGO_VERSION) { $env:DBGO_VERSION } else { "dev" })
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Root = Split-Path -Parent $ScriptDir
Set-Location $Root

if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    throw "未找到 npm"
}
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "未找到 go"
}

Push-Location (Join-Path $Root "web")
try {
    if (Test-Path "package-lock.json") {
        npm ci
    } else {
        throw "缺少 web/package-lock.json"
    }
    if ($LASTEXITCODE -ne 0) { throw "npm ci 失败" }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw "前端构建失败" }
} finally {
    Pop-Location
}

$commit = "none"
try {
    $commit = (git -C $Root rev-parse --short HEAD).Trim()
} catch {
    $commit = "none"
}
$built = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = "-s -w -X github.com/cng1985/dbgo/internal/platform/buildinfo.Version=$Version -X github.com/cng1985/dbgo/internal/platform/buildinfo.Commit=$commit -X github.com/cng1985/dbgo/internal/platform/buildinfo.Time=$built"
$env:CGO_ENABLED = "0"
$env:GOTOOLCHAIN = "local"

$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Name = "dbgo.exe" },
    @{ GOOS = "linux"; GOARCH = "amd64"; Name = "dbgo" },
    @{ GOOS = "linux"; GOARCH = "arm64"; Name = "dbgo" }
)

$release = Join-Path $Root "release"
if (Test-Path $release) {
    Remove-Item -Recurse -Force $release
}
New-Item -ItemType Directory -Path $release | Out-Null

foreach ($target in $targets) {
    $folderName = "$($target.GOOS)-$($target.GOARCH)"
    $folder = Join-Path $release $folderName
    New-Item -ItemType Directory -Path $folder | Out-Null
    $binary = Join-Path $folder $target.Name
    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    & go build -trimpath -ldflags $ldflags -o $binary ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw "编译 $folderName 失败" }

    Copy-Item -Recurse (Join-Path $Root "configs") (Join-Path $folder "configs")
    Copy-Item -Recurse (Join-Path $Root "migrations") (Join-Path $folder "migrations")
    if ($target.GOOS -eq "linux") {
        Copy-Item (Join-Path $Root "scripts/install-linux.sh") (Join-Path $folder "install-linux.sh")
    }
    Set-Content -Path (Join-Path $folder "VERSION.txt") -Value "dbgo $Version commit $commit built $built" -Encoding utf8

    $hash = Get-FileHash -Algorithm SHA256 $binary
    Set-Content -Path (Join-Path $folder "SHA256SUMS") -Value "$($hash.Hash.ToLower())  $($target.Name)" -Encoding ascii
}

Write-Host "发布包已写入 $release"
