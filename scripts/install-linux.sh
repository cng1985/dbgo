#!/bin/sh
# 把当前目录中的 dbgo 安装到 /opt/dbgo，数据放在 /var/lib/dbgo。
# 需要 root。更新时停止服务、替换二进制，不覆盖已有配置和数据。
set -eu

APP_USER=dbgo
APP_DIR=/opt/dbgo
DATA_DIR=/var/lib/dbgo
UNIT=/etc/systemd/system/dbgo.service
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

if [ "$(id -u)" -ne 0 ]; then
  echo "请用 root 运行安装脚本" >&2
  exit 1
fi

arch=$(uname -m)
case "$arch" in
  x86_64) expect=amd64 ;;
  aarch64|arm64) expect=arm64 ;;
  *)
    echo "不支持的架构: $arch" >&2
    exit 1
    ;;
esac

if [ ! -f "$SCRIPT_DIR/dbgo" ]; then
  echo "当前目录缺少 dbgo 二进制" >&2
  exit 1
fi
if [ ! -d "$SCRIPT_DIR/configs" ]; then
  echo "当前目录缺少 configs" >&2
  exit 1
fi

if ! id "$APP_USER" >/dev/null 2>&1; then
  useradd --system --home-dir "$DATA_DIR" --create-home --shell /usr/sbin/nologin "$APP_USER"
fi

install -d -m 0755 "$APP_DIR"
install -d -m 0750 -o "$APP_USER" -g "$APP_USER" "$DATA_DIR"
install -d -m 0750 -o "$APP_USER" -g "$APP_USER" "$DATA_DIR/backups"

if [ ! -d "$APP_DIR/configs" ]; then
  cp -a "$SCRIPT_DIR/configs" "$APP_DIR/configs"
  chown -R root:root "$APP_DIR/configs"
  chmod 0755 "$APP_DIR/configs"
  chmod 0644 "$APP_DIR/configs"/*
fi

updating=0
if [ -f "$UNIT" ]; then
  updating=1
  systemctl stop dbgo
fi

install -m 0755 "$SCRIPT_DIR/dbgo" "$APP_DIR/dbgo.next"
mv -f "$APP_DIR/dbgo.next" "$APP_DIR/dbgo"

if [ -n "${APP_AUTH_ADMIN_PASSWORD:-}" ]; then
  install -d -m 0750 /etc/systemd/system/dbgo.service.d
  umask 077
  {
    printf '%s\n' '[Service]'
    printf 'Environment=APP_AUTH_ADMIN_PASSWORD=%s\n' "$(printf '%s' "$APP_AUTH_ADMIN_PASSWORD" | sed 's/%/%%/g')"
  } > /etc/systemd/system/dbgo.service.d/admin.conf
  chmod 0600 /etc/systemd/system/dbgo.service.d/admin.conf
fi

cat > "$UNIT" <<EOF
[Unit]
Description=DBGo database console
After=network.target

[Service]
Type=simple
User=$APP_USER
Group=$APP_USER
WorkingDirectory=$DATA_DIR
ExecStart=$APP_DIR/dbgo --config-dir $APP_DIR/configs --data-dir $DATA_DIR --profile pro
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
if [ "$updating" -eq 0 ] && [ ! -f "$DATA_DIR/app.db" ] && [ -z "${APP_AUTH_ADMIN_PASSWORD:-}" ]; then
  systemctl enable dbgo
  echo "程序已安装，但生产环境首次启动必须提供 APP_AUTH_ADMIN_PASSWORD，因此没有启动服务。"
  echo "设置后执行: systemctl restart dbgo"
  exit 0
fi
if [ "$updating" -eq 0 ]; then
  systemctl enable --now dbgo
else
  systemctl restart dbgo
fi

systemctl is-active --quiet dbgo
echo "dbgo 已安装，当前机器架构对应 $expect。查看状态: systemctl status dbgo"
echo "查看日志: journalctl -u dbgo"
