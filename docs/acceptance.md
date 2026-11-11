# 验收

检查日期：2026-11-10。工具链：Go 1.22.2，Node 22。应用库驱动是 `github.com/glebarez/sqlite`（modernc.org/sqlite）。目标库驱动是 `github.com/go-sql-driver/mysql`、`github.com/jackc/pgx/v5` 和 `github.com/microsoft/go-mssqldb`。

## 支持矩阵

| 数据库 | 应用库迁移 | 目标库查询、目录、迁移、备份 | 证据 |
| --- | --- | --- | --- |
| SQLite | 已验证 | 已验证 | `go test ./internal/app/` 的控制台流程，以及连接删除回滚测试 |
| MySQL | 计划支持 | 计划支持 | 驱动可编译；标识符和危险语句有单元测试。本环境没有 MySQL 服务 |
| PostgreSQL | 计划支持 | 计划支持 | 同上，另有占位符改写测试。本环境没有 PostgreSQL 服务 |
| SQL Server | 计划支持 | 计划支持 | 驱动可编译，标识符有单元测试。本环境没有 SQL Server 服务 |

`windows/amd64`、`linux/amd64`、`linux/arm64` 可以由 `CGO_ENABLED=0` 交叉编译。只有当前 Linux amd64 进程被测试启动过。

## 规范条目

| 编号 | 结果 |
| --- | --- |
| A01 | 控制台已拆成 connection、query、catalog、migration、backup、overview、llm、monitor、snippet。父模块只组合。没有 Service 层 |
| A02 | Resource 编排，Repository 管应用库，目标库走 `shared.Opener` |
| A03 | 公开业务方法使用一个 Request 或 Search，返回 Response 或 PageResponse |
| A04 | 连接更新和删除在 Repository 事务内完成 |
| A05 | 删除连接与删除迁移定义共用一个事务。目标库语句与应用库状态是补偿，不是跨库事务 |
| A06 | 受保护接口要求 Scope。admin 与 member 都能使用控制台，但只能看到自己创建的连接 |
| A07 | 空备注可以保存。更新密码留空会保持原密文。主键用字符串 |
| A08 | 连接排序有白名单。关键字搜索转义 LIKE |
| A09 | 三份配置在 `configs/`。环境变量 `false` 和端口 `0` 有测试 |
| A10 | 默认 SQLite 有真库测试。另外三库只计划支持，见上表 |
| A11 | 未知 API、页面回退、缺失静态文件、未登录和健康检查在流程测试里覆盖 |
| A12 | 三个脚本入口已提供。发布包在执行脚本的环境生成。systemd 安装没有在真实主机上演练 |
| A13 | SQLite SQL 备份和恢复在流程测试里通过。升级失败后的二进制回切没有单独演练 |
| A14 | 没有引入队列、缓存或对象存储 |
| A15 | 不实现商品样板。连接的不存在、相同值更新、空备注、重复删除、空分页、非法页大小、未登录和版本冲突由流程测试覆盖。迁移执行中删除连接的回滚由 `connection/delete_test.go` 覆盖 |

## 明确决策

- 认证不能关闭。没有匿名 SQL 控制台。
- GORM 关闭默认事务，避免每条写入都多一层事务。
- 连接按创建人隔离。注册成员与管理员各自看到自己的连接，模型配置仍然共享。
- 连接可以导出。默认文件不含口令；`include_password` 为 true 时才写入口令。仍可导入 `dbgo-connections` 文件。
- 代码段按创建人隔离，可以导入导出 `dbgo-snippets` 文件。
- 编辑表定义时返回全部字段。类型保留长度和精度，并带上数据库里的字段注释。
- 编辑已有表时不执行建表定义。按字段差异生成更新语句，弹出确认后才执行。
- 页码或页大小为 0 时使用默认值，因为 JSON 分不出缺省和 0。
- 管理员初始口令不写进仓库，也不写进日志。
