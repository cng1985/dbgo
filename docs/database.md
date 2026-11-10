# 数据库

## 应用库

应用库保存控制台自己的数据，不保存目标库里的业务表。默认驱动是 SQLite，文件是数据目录下的 `app.db`。也可以把应用库换成 MySQL、PostgreSQL 或 SQL Server，但必须同时给出 DSN，连接失败会中止启动。

启动时按顺序执行 `migrations/0001_init.<driver>.sql` 和 `migrations/0002_studio.<driver>.sql`。脚本按驱动嵌入二进制。已执行版本记在 `schema_migration`，校验值不一致会拒绝启动。迁移期间按驱动加锁：SQLite 使用 `BEGIN IMMEDIATE`，MySQL 使用 `GET_LOCK`，PostgreSQL 使用 advisory lock，SQL Server 使用 `sp_getapplock`。

| 表 | 内容 |
| --- | --- |
| `app_user` | 管理员和注册成员。口令是 bcrypt，邮箱可空 |
| `llm_profile` | OpenAI 协议模型。密钥是密文，名称唯一 |
| `app_session` | 会话令牌哈希、过期和撤销时间 |
| `db_connection` | 目标库连接。口令字段是密文，版本从 1 起 |
| `query_history` | SQL 文本、状态、耗时和行数，不保存结果网格 |
| `data_migration` | 目标库迁移脚本、校验值和状态 |
| `data_backup` | 备份记录。文件在数据目录 `backups/` |

SQLite 打开后会再次设置 `foreign_keys`、`busy_timeout` 和 `journal_mode`。默认 journal mode 是 `DELETE`，避免只复制主库文件时漏掉 WAL。应用库连接池最大 1。GORM 关闭默认事务，多步写入使用显式事务。

## 目标库

连接里的驱动：

| 值 | 别名 | 默认端口 |
| --- | --- | --- |
| sqlite | | |
| mysql | | 3306 |
| postgres | pgsql、postgresql | 5432 |
| sqlserver | mssql | 1433 |

SQLite 路径相对数据目录时不能包含 `..`，也不能指向应用库文件。绝对路径允许，但仍不能是应用库。网络库的主机不能包含空格或斜线。

查询默认只读。PostgreSQL 在事务里设置 `READ ONLY`。MySQL 的迁移和恢复不包进一个事务，因为 DDL 会隐式提交；其他驱动的迁移和 SQL 恢复使用事务。PostgreSQL 的 `?` 会在字符串字面量之外改成 `$1` 形式。

目录查询使用 `information_schema` 或 SQLite 的 `pragma`，标识符按驱动加引号，过滤值使用参数。

SQL 备份按表导出 `CREATE` 和分批 `INSERT`，最多 200 张表。MySQL 备份开始时设置 `NO_BACKSLASH_ESCAPES`，字符串只用成对单引号。SQLite 的二进制副本使用 `VACUUM INTO`，只能下载，不能在线恢复。

## 一致性

应用库和目标库不能组成一个事务。迁移和恢复的状态变化是补偿式的：先占住中间状态，成功后写终态，失败则写失败或退回原状态。中间状态留下时，用 resolve 的 abort 或 confirm 处理，不把未确认的脚本标成已完成。

删除连接和删除它的迁移定义在应用库的同一个事务里。迁移仍在执行或回滚时，整个删除回滚。
