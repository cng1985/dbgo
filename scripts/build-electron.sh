#!/usr/bin/env bash
# 构建前端和各平台服务程序，再打出 Windows、macOS、Linux 的 Electron 包。
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

version="${DBGO_VERSION:-0.1.0}"
current_only=0
if [[ "${1:-}" == "--current" ]]; then
  current_only=1
fi

if ! command -v npm >/dev/null 2>&1; then
  echo "未找到 npm" >&2
  exit 1
fi
if ! command -v go >/dev/null 2>&1; then
  echo "未找到 go" >&2
  exit 1
fi

npm --prefix web ci
npm --prefix web run build

commit="none"
if git -C "$root" rev-parse --short HEAD >/dev/null 2>&1; then
  commit="$(git -C "$root" rev-parse --short HEAD)"
fi
built="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
ldflags="-s -w -X github.com/cng1985/dbgo/internal/platform/buildinfo.Version=${version} -X github.com/cng1985/dbgo/internal/platform/buildinfo.Commit=${commit} -X github.com/cng1985/dbgo/internal/platform/buildinfo.Time=${built}"

resources="$root/electron/resources"
rm -rf "$resources"
mkdir -p "$resources"
cp -a "$root/configs" "$resources/configs"

compile_one() {
  local folder="$1"
  local goos="$2"
  local goarch="$3"
  local name="$4"
  local out="$resources/$folder"
  mkdir -p "$out"
  echo "编译 $folder"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOTOOLCHAIN=local \
    go build -trimpath -ldflags "$ldflags" -o "$out/$name" ./cmd/server
}

host_folder() {
  local goos goarch folder name
  goos="$(go env GOOS)"
  goarch="$(go env GOARCH)"
  case "$goos" in
    windows) folder="win"; name="dbgo.exe" ;;
    darwin) folder="mac"; name="dbgo" ;;
    linux) folder="linux"; name="dbgo" ;;
    *) echo "不支持的系统 $goos" >&2; exit 1 ;;
  esac
  case "$goarch" in
    amd64) goarch="x64" ;;
    arm64) ;;
    *) echo "不支持的架构 $goarch" >&2; exit 1 ;;
  esac
  printf '%s-%s %s %s %s\n' "$folder" "$goarch" "$goos" "$(go env GOARCH)" "$name"
}

if [[ "$current_only" -eq 1 ]]; then
  read -r folder goos goarch name <<<"$(host_folder)"
  compile_one "$folder" "$goos" "$goarch" "$name"
else
  compile_one win-x64 windows amd64 dbgo.exe
  compile_one mac-x64 darwin amd64 dbgo
  compile_one mac-arm64 darwin arm64 dbgo
  compile_one linux-x64 linux amd64 dbgo
  compile_one linux-arm64 linux arm64 dbgo
fi

stage_bin() {
  local folder="$1"
  rm -rf "$resources/bin"
  mkdir -p "$resources/bin"
  cp -a "$resources/$folder/." "$resources/bin/"
  if [[ -f "$resources/bin/dbgo" ]]; then
    chmod +x "$resources/bin/dbgo"
  fi
}

npm --prefix electron ci
export CSC_IDENTITY_AUTO_DISCOVERY=false

packaged_binary() {
  local folder="$1"
  case "$folder" in
    win-x64) printf '%s\n' "$root/electron/dist/win-unpacked/resources/bin/dbgo.exe" ;;
    mac-x64) printf '%s\n' "$root/electron/dist/mac/DBGo.app/Contents/Resources/bin/dbgo" ;;
    mac-arm64) printf '%s\n' "$root/electron/dist/mac-arm64/DBGo.app/Contents/Resources/bin/dbgo" ;;
    linux-x64) printf '%s\n' "$root/electron/dist/linux-unpacked/resources/bin/dbgo" ;;
    linux-arm64) printf '%s\n' "$root/electron/dist/linux-arm64-unpacked/resources/bin/dbgo" ;;
    *) echo "没有 $folder 的安装包路径" >&2; exit 1 ;;
  esac
}

verify_binary() {
  local folder="$1"
  local bin info
  bin="$(packaged_binary "$folder")"
  if [[ ! -f "$bin" ]]; then
    echo "缺少服务程序 $bin" >&2
    exit 1
  fi
  info="$(file "$bin")"
  echo "$info"
  case "$folder" in
    win-x64) [[ "$info" == *"PE32+"* && "$info" == *"x86-64"* ]] ;;
    mac-x64) [[ "$info" == *"Mach-O"* && "$info" == *"x86_64"* ]] ;;
    mac-arm64) [[ "$info" == *"Mach-O"* && "$info" == *"arm64"* ]] ;;
    linux-x64) [[ "$info" == *"ELF"* && "$info" == *"x86-64"* ]] ;;
    linux-arm64) [[ "$info" == *"ELF"* && "$info" == *"aarch64"* ]] ;;
  esac || {
    echo "$folder 的服务程序架构不对" >&2
    exit 1
  }
}

pack_one() {
  local folder="$1"
  shift
  stage_bin "$folder"
  echo "打包 $folder"
  (cd "$root/electron" && npx electron-builder "$@")
  verify_binary "$folder"
}

if [[ "$current_only" -eq 1 ]]; then
  read -r folder goos goarch name <<<"$(host_folder)"
  case "$goos" in
    windows) pack_one "$folder" --win --x64 ;;
    darwin)
      if [[ "$goarch" == "arm64" ]]; then
        pack_one "$folder" --mac --arm64
      else
        pack_one "$folder" --mac --x64
      fi
      ;;
    linux)
      if [[ "$goarch" == "arm64" ]]; then
        pack_one "$folder" --linux --arm64
      else
        pack_one "$folder" --linux --x64
      fi
      ;;
  esac
else
  pack_one win-x64 --win --x64
  pack_one mac-x64 --mac --x64
  pack_one mac-arm64 --mac --arm64
  pack_one linux-x64 --linux --x64
  pack_one linux-arm64 --linux --arm64
fi

echo "Electron 发行包在 $root/electron/dist"
