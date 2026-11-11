# DBGo

网页版数据库工作台。用来保存连接、设计表和例程、执行 SQL、查看性能、用兼容 OpenAI 协议的模型优化语句，以及做版本化迁移和备份恢复。应用自己的账号、会话和元数据默认放在 SQLite。目标库支持 SQLite、MySQL、PostgreSQL 和 SQL Server。

结构遵循《Go 项目结构规范》v3.4：Gin、GORM、Fx，前端是 Vue 3 + JavaScript + Vite。业务没有 Service 层。规范第 20 章的商品样板不在本产品里，对应的增删改查和事务验收由连接管理承担。

## 启动

需要 Go 1.22。首次启动会在数据目录生成管理员口令文件，生产环境则要求事先设置口令。

```bash
go run ./cmd/server --data-dir ./data
```

开发环境打开文本日志：

```bash
APP_PROFILE=dev go run ./cmd/server --data-dir ./data
```

生产环境：

```bash
APP_PROFILE=pro APP_AUTH_ADMIN_PASSWORD='至少八位' go run ./cmd/server --data-dir ./data
```

默认监听 `127.0.0.1:8080`。浏览器打开该地址，使用 `admin` 和首次生成的口令登录，也可以注册成员账号。口令文件是 `data/initial-admin-password.txt`，权限 `0600`，日志只记录路径。`auth.allow_register` 为 false 时关闭注册。

常用参数：

| 参数 | 环境变量 | 作用 |
| --- | --- | --- |
| `--base-dir` | `APP_BASE_DIR` | 程序根目录 |
| `--config-dir` | `APP_CONFIG_DIR` | 配置目录，默认 `<base>/configs` |
| `--data-dir` | `APP_DATA_DIR` | 数据目录，默认 `<base>/data` |
| `--profile` | `APP_PROFILE` | 空、`dev` 或 `pro` |
| `--set key=value` | 同名环境变量 | 覆盖单个配置，键与环境变量相同 |
| `--version` | | 打印版本、提交号和构建时间 |

覆盖顺序是基础配置、profile 配置、环境变量、`--set`。布尔值只接受 `true` 或 `false`。空的口令、DSN 或密钥环境变量视为错误。

## 配置

三份配置都在 `configs/`：

- `application.yaml` 是默认值，不含口令、DSN 和密钥。
- `application-dev.yaml` 使用 debug 文本日志。
- `application-pro.yaml` 把会话 Cookie 标为 Secure，并拒绝 debug 日志。

应用库默认是数据目录里的 `app.db`。改用 MySQL、PostgreSQL 或 SQL Server 时设置 `APP_DATABASE_DRIVER` 和 `APP_DATABASE_DSN`，未知驱动或连接失败会中止启动，不会退回 SQLite。控制台保存的是目标库凭据，因此 `database.enabled=false` 和 `auth.enabled=false` 都会被拒绝。

字段、默认值和环境变量见 [docs/database.md](docs/database.md) 与 [docs/deployment.md](docs/deployment.md)。

## 页面

登录后进入数据库工作台：左侧是数据库导航，中间是多标签 SQL 编辑器，底部是状态栏。菜单和工具栏打开其余功能。

- 导航按连接、模式、表、视图、存储过程、函数、字段和索引展开，可按名称过滤。双击表会打开只读查询，双击例程会打开定义。
- SQL 编辑器会高亮语法，并按所选连接和模式提示关键字、表和字段。编辑器和结果网格支持右键：执行选区、格式化、解释、注释、复制，以及按单元格生成 INSERT 或筛选。默认只读，写入要取消只读。
- 可以新建并编辑表、视图、存储过程和函数。SQLite 没有存储过程和函数。
- 可以登记兼容 OpenAI 协议的模型，并对当前 SQL 给出改写建议。
- 性能页读取当前连接的库大小、会话和表规模。
- 连接可以导出成 JSON，再按跳过、改名或报错导入。默认不导出密码。
- 总览查看连接、查询、失败查询、迁移和备份数量。个人中心可以改显示名和邮箱。
- 可以新建并测试 SQLite、MySQL、PostgreSQL、SQL Server 连接。每个账号只能看到自己创建的连接。
- 可以编写、执行和回滚迁移。失败的脚本保持失败状态。
- 可以创建 SQL 备份并恢复。SQLite 还可以做 `VACUUM INTO` 文件副本，该副本只供下载。

接口说明见 [docs/api.md](docs/api.md)。

## 构建与安装

```bash
npm --prefix web ci
npm --prefix web run build
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...
```

发布包由 `scripts/build-platforms.ps1` 生成，Windows 入口是 `scripts/build-platforms.cmd`。脚本安装前端依赖、构建一次页面，再编译 `windows/amd64`、`linux/amd64`、`linux/arm64`。产物在 `release/<系统>-<架构>/`，包含二进制、配置模板、迁移 SQL、版本说明和校验值。不包含数据库、口令和日志。

Linux 安装：

```bash
sudo APP_AUTH_ADMIN_PASSWORD='至少八位' ./install-linux.sh
```

程序在 `/opt/dbgo`，数据在 `/var/lib/dbgo`，服务用户是 `dbgo`，profile 是 `pro`。更新会先停止服务，再用新文件替换二进制，已有配置和数据保留。

```bash
systemctl status dbgo
journalctl -u dbgo
```

## 健康检查

- `GET /health/live` 返回 HTTP 200，`{"status":"live"}`。
- `GET /health/ready` 在迁移和数据库可用时返回 HTTP 200，否则 HTTP 503。

这两个地址不走 JSON API 的“业务错误也返回 200”约定。

## 验证

```bash
node --test web/src/sqlEditor.test.js
GOTOOLCHAIN=local go test ./...
```

`internal/app/flow_test.go` 用真实 SQLite 走登录、连接、查询、目录、迁移、备份恢复和 HTTP 形状。MySQL、PostgreSQL、SQL Server 在本环境没有可用服务，只验证驱动可以编译以及标识符、占位符规则。矩阵见 [docs/acceptance.md](docs/acceptance.md)。
