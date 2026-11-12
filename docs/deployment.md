# 部署

## 配置字典

时长必须带单位，例如 `30s`、`2m`、`8h`。列表环境变量使用 JSON 数组。布尔值只有 `true` 和 `false`，`false` 和 `0` 会覆盖模板里的真值。空字符串不能用来清空 DSN、管理员口令或密钥。

| YAML | 环境变量 / `--set` | 默认 |
| --- | --- | --- |
| `app.name` | `APP_NAME` | dbgo |
| `app.data_dir` | `APP_DATA_DIR` 或 `--data-dir` | `./data` |
| `server.host` | `APP_SERVER_HOST` | `127.0.0.1` |
| `server.port` | `APP_SERVER_PORT` | 8080 |
| `server.request_timeout` | `APP_SERVER_REQUEST_TIMEOUT` | 30s |
| `server.long_request_timeout` | `APP_SERVER_LONG_REQUEST_TIMEOUT` | 2m |
| `server.rate_per_minute` | `APP_SERVER_RATE_PER_MINUTE` | 120 |
| `server.trusted_proxies` | `APP_SERVER_TRUSTED_PROXIES` | `[]`，不信任代理 |
| `database.driver` | `APP_DATABASE_DRIVER` | sqlite |
| `database.dsn` | `APP_DATABASE_DSN` | 空。网络驱动必填 |
| `database.sqlite.path` | `APP_DATABASE_SQLITE_PATH` | app.db |
| `database.sqlite.foreign_keys` | `APP_DATABASE_SQLITE_FOREIGN_KEYS` | true |
| `auth.admin_username` | `APP_AUTH_ADMIN_USERNAME` | admin |
| `auth.admin_password` | `APP_AUTH_ADMIN_PASSWORD` | 空。开发环境可自动生成，生产环境必填 |
| `auth.secret` | `APP_AUTH_SECRET` | 空则生成 `data/auth.key` |
| `auth.login_rate_per_minute` | `APP_AUTH_LOGIN_RATE_PER_MINUTE` | 10 |
| `query.max_rows` | `APP_QUERY_MAX_ROWS` | 1000，允许 1 到 1000 |
| `query.max_sql_bytes` | `APP_QUERY_MAX_SQL_BYTES` | 100000 |
| `backup.max_rows` | `APP_BACKUP_MAX_ROWS` | 50000 |
| `backup.max_bytes` | `APP_BACKUP_MAX_BYTES` | 33554432 |

其余超时、连接池和日志字段使用同样的命名：把 YAML 路径改成大写并用下划线连接，加上 `APP_` 前缀。`server.long_request_timeout` 不能小于 `server.request_timeout`。生产 profile 不能把日志级别设为 debug。

`database.enabled` 和 `auth.enabled` 必须为 true。控制台要保存连接凭据并执行 SQL，这两项不能关闭。

## 日志

默认 JSON 打到标准输出，带服务名、版本、请求号、操作者、路由、状态和耗时。含口令字样的数据库语句不写原文。不要把 DSN、口令或密钥写进配置文件。

## 发布

在仓库根目录执行：

```powershell
./scripts/build-platforms.ps1 -Version 0.1.0
```

Windows CMD 使用 `scripts\build-platforms.cmd`。脚本不依赖当前工作目录。前端只构建一次，三个目标都嵌入同一份页面。`CGO_ENABLED=0`。版本通过 `-ldflags` 写入 `internal/platform/buildinfo`。

`release/linux-amd64/` 和 `release/linux-arm64/` 带有 `install-linux.sh`。安装到 `/opt/dbgo`，数据目录 `/var/lib/dbgo`，运行用户 `dbgo`，profile 为 `pro`。更新先停服务，把新二进制写到临时文件再改名，不覆盖已有 `configs` 和数据。

本环境已用 Go 1.22 交叉编译三个目标。没有在 Windows 或 ARM 机器上启动过这些二进制，安装脚本也没有在 systemd 主机上实际跑过。

## 桌面发行

Windows 在仓库里执行 `scripts\build-electron.cmd`。它调用 PowerShell，不需要 Git Bash 或 WSL。Linux 和 macOS 执行：

```bash
./scripts/build-electron.sh
```

两条命令都先构建页面，再用 `CGO_ENABLED=0` 交叉编译服务程序，然后用 Electron 打出这些包：

| 系统 | 架构 | 产物 |
| --- | --- | --- |
| Windows | x64 | `DBGo-*-win.zip` |
| macOS | x64、arm64 | `DBGo-*-mac.zip` |
| Linux | x64、arm64 | `dbgo-*.tar.gz`；在 Linux 上还会生成 `DBGo-*.AppImage` |

产物在 `electron/dist/`。macOS 包未签名。Windows 是压缩包，不需要安装程序。只编当前系统时加 `--current`。

在 Windows 上可以打出 Windows zip、macOS zip 和 Linux tar.gz。AppImage 的打包工具是 Linux 程序，所以只在 Linux 上生成。Windows 打出的 macOS 和 Linux 压缩包会补上 `DBGo` 和 `dbgo` 的可执行权限。若 macOS 打包提示无法创建符号链接，先在 Windows 设置中打开开发人员模式。需要已安装 Node.js 和 Go。

桌面窗口监听 `127.0.0.1` 的空闲端口，不使用生产 profile，因此会话 Cookie 可以在本机页面上保存。启动时丢掉继承的 `APP_` 环境变量，应用库固定为用户目录里的 SQLite，不会沿用本机服务的数据库连接。数据目录在 Electron 的 `userData/data`，日志在 `userData/logs/dbgo.log`。第一次启动生成 `initial-admin-password.txt`，并用对话框给出 `admin` 和口令。关闭窗口时结束服务程序。Windows 压缩包不签名，在 Linux 上打包不需要 Wine。

本机已用 Linux x64 的解包目录启动窗口，登录后进入工作台，关闭窗口后本机服务一起退出。Windows 和 macOS 的压缩包是在 Linux 上生成的，并用 `file` 核对过里面的服务程序分别是 Windows x64、macOS x64 和 macOS arm64。没有在 Windows 或 Mac 上打开过这些包。

## 健康、关闭和备份

存活和就绪见 README。关闭时先把就绪标成不可用，再在 `server.shutdown_timeout` 内排空 HTTP，然后关闭应用库。

应用库若使用 SQLite，在线备份应使用控制台的 SQLite 副本或先停止写入。不要在 WAL 模式下只复制 `app.db`。目标库的 SQL 备份是逻辑导出，恢复会再次执行其中的语句，所以恢复前要确认目标库可以被这些 `CREATE` 和 `INSERT` 覆盖。备份成功只说明文件写完，恢复是否可用要单独演练。

限流是单进程内存窗口。API 默认每分钟 120 次，登录 10 次。多实例不会共享计数。
