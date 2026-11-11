# 架构

DBGo 是单进程控制台。`cmd/server/main.go` 只调用 `app.Run()`。平台层负责配置、日志、应用库、认证、HTTP 和统一响应。`internal/modules/console` 只组合子模块，不写业务。

```text
cmd/server
internal/app
internal/platform
  config database domain response errors middleware routing server logging auth secret sqlscript buildinfo
internal/modules/console
  connection query catalog migration backup overview llm monitor snippet shared
web
migrations
configs
```

## 请求形状

公开业务方法都是一个 Request 或 Search 进，`response.Response[T]` 或 `response.PageResponse[T]` 加 error 出。Controller 只做绑定、调用和输出。Resource 做校验、权限和编排。Repository 做数据读写；多步操作自己开启事务。没有 Service 层。

普通 JSON 接口成功和业务错误都返回 HTTP 200。响应体里的 `code` 表示结果。分页响应只有 `data`、`total`、`page`、`page_size`。未知 `/api` 返回 HTTP 404 JSON，方法不匹配返回 HTTP 405 JSON。页面路由回退到 `index.html`，缺失的静态文件返回真实 404。

主键在 JSON 里是十进制字符串。页码和页大小小于 1 时使用 1 和 20；页大小超过 200 返回错误。

## 两个数据库

应用库保存用户、会话、连接、查询历史、迁移定义、备份记录和代码段。目标库是用户登记的 MySQL、PostgreSQL、SQL Server 或 SQLite。目标库用 `database/sql` 打开，每次操作新建最多 2 个连接的小池，用完关闭，不缓存。

应用库使用 GORM，并设置 `SkipDefaultTransaction: true`。单条写入不再套一层默认事务；跨步骤的更新、删除和迁移清理显式调用 `Transaction`。SQLite 应用库的最大连接数是 1。

删除连接时，connection 的 Repository 开启事务，调用 migration 的 `DeleteOwnedWithTx`。两边使用同一个 `*gorm.DB`。有 `applying` 或 `reverting` 的迁移时拒绝删除，连接和迁移都保留。查询历史和备份不删除，因为它们保存的是当时的名称快照。

目标库上的 SQL、迁移和恢复与应用库不在同一个事务里。迁移先把状态改成 `applying` 或 `reverting`，语句成功后再改成终态。若目标库已经执行但状态没写上，记录会停在中间态，页面可以用“中止”或“确认”收口。

## 认证和范围

会话放在 HttpOnly Cookie `dbgo_session`。所有 `/api` 请求还要带 `X-DBGo-Request: 1`，并在提供 Origin 时与 Host 一致。登录口令使用 bcrypt。会话表只保存令牌的 SHA256。

上下文里的 Scope 含操作者、请求号和角色。`admin` 与注册产生的 `member` 都能使用控制台。连接按 `created_by` 隔离：列表、打开、修改、删除、导入、导出，以及这条连接上的查询、迁移和备份，都只对创建人可见。导出默认不包含口令，只有请求显式要求时才解密写入文件。代码段同样只属于创建人。两人可以使用相同的连接名称和代码段名称。模型配置仍然共享。

连接口令用 AES-GCM 加密。密钥来自 `APP_AUTH_SECRET`，否则使用数据目录里自动生成的 `auth.key`。

## 前端

`web/` 是 Vue 3、JavaScript、Vite 和 Pinia。登录或注册后是数据库工作台：菜单和工具栏、左侧对象树、多标签 SQL 编辑器、对象设计、代码段、性能、模型配置和个人中心。构建结果放进 `web/dist`，由 `web/embed.go` 嵌进二进制。页面不内联脚本和样式，以匹配 `default-src 'self'`。查询编辑器的高亮和补全也写在页面样式里，不在运行时注入 `<style>`。接口封装区分普通响应和分页响应，主键保持字符串。
