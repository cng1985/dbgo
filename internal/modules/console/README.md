# 控制台模块

DBGo 控制台按业务能力拆成子模块。父目录只做组合，子模块不反向导入本包。

| 子模块 | 职责 | 数据所有权 |
| --- | --- | --- |
| connection | 保存并测试 MySQL、PostgreSQL、SQL Server、SQLite 连接 | `db_connection` |
| query | 在选定连接上执行 SQL，并保留查询历史 | `query_history` |
| catalog | 查看模式、表、字段和索引，生成并执行表、视图、存储过程和函数的 DDL | 不落本地库 |
| migration | 管理目标库的版本化升级与回滚脚本 | `data_migration` |
| backup | 生成、下载、恢复逻辑备份；SQLite 另支持一致的二进制副本 | `data_backup` 与 `data/backups` |
| overview | 汇总数量和最近查询 | 不拥有表，只调用其他模块的查询 |
| llm | 保存 OpenAI 协议模型，并用当前库结构给出 SQL 改写建议 | `llm_profile` |
| monitor | 读取目标库的大小、会话和表规模 | 不落本地库 |
| snippet | 保存当前用户的 SQL 代码段，并支持导入导出 | `sql_snippet` |

允许的依赖：

- query、catalog、migration、backup、llm、monitor 通过 `shared.Opener` 打开目标库，由 connection 提供实现。
- connection 负责连接 JSON 的导入和导出。导出默认不含口令。
- snippet 只读写当前用户自己的代码段。
- 连接按创建人隔离。migration、backup 通过 `Describe` 读取当前用户自己的连接名称，不写连接表。
- 删除连接是跨模块原子操作：connection 的 Repository 开启事务，migration 的 `DeleteOwnedWithTx` 使用同一个事务删除该连接的迁移定义。执行中的迁移会拒绝删除。查询历史和备份文件保留快照名称，不随连接删除。

目标库上的 SQL、迁移和恢复不与应用元数据库处于同一个事务。状态写入失败时保留 `applying` 或 `reverting`，由 resolve 动作人工确认。
