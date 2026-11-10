# 接口

除下载和健康检查外，接口都是 `POST`，请求体是 JSON，需要已登录 Cookie，并带请求头 `X-DBGo-Request: 1`。

普通成功：

```json
{"code":200,"msg":"success","data":{}}
```

分页成功没有 `code` 和 `msg`：

```json
{"data":[],"total":0,"page":1,"page_size":20}
```

业务错误仍然是 HTTP 200，例如 `{"code":404,"msg":"连接不存在"}`。模块错误码：

| 码 | 范围 |
| --- | --- |
| 10001 | 连接 |
| 10002 | SQL 与目录 |
| 10003 | 迁移 |
| 10004 | 备份 |

主键字段是字符串。时间是 RFC3339。

## 认证

| 路径 | 说明 |
| --- | --- |
| `POST /api/auth/login` | 公开。成功后写 Cookie，响应不包含会话令牌 |
| `POST /api/auth/logout` | 撤销当前会话并清除 Cookie |
| `POST /api/auth/profile` | 当前用户 |
| `POST /api/auth/password` | `current_password`、`new_password`。成功后所有会话失效 |

## 总览与连接

| 路径 | 说明 |
| --- | --- |
| `POST /api/overview/one` | 数量和最近 5 条查询 |
| `POST /api/connection/create` | 新建。客户端不能指定 id |
| `POST /api/connection/update` | 全量更新，必须带 `id` 和 `version`。密码空字符串表示保持原值，备注空字符串会清空 |
| `POST /api/connection/one` | 单条 |
| `POST /api/connection/search` | 分页。`keyword`、`driver`、`sort`、`direction` |
| `POST /api/connection/delete` | 不存在返回 404。重复删除仍是 404 |
| `POST /api/connection/test` | 长请求。返回版本和耗时 |

SQLite 的 `sqlite_path` 可以是相对数据目录的路径。网络库需要主机、库名和用户名，端口 0 使用默认端口。PostgreSQL 的 `ssl_mode` 可以是 `disable`、`require`、`verify-ca`、`verify-full`。

## 查询与目录

| 路径 | 说明 |
| --- | --- |
| `POST /api/query/execute` | `connection_id`、`sql`、`read_only`、`max_rows`、`timeout_seconds`。`read_only` 省略时为 true |
| `POST /api/query/one` | 历史详情，不含结果网格 |
| `POST /api/query/search` | 可按连接和 `success`/`failed` 筛选 |
| `POST /api/catalog/schemas` | 模式 |
| `POST /api/catalog/tables` | 表 |
| `POST /api/catalog/columns` | 字段 |
| `POST /api/catalog/indexes` | 索引 |

单次查询最多 50 条语句。结果行数默认 100，不超过配置上限。`load_extension`、`xp_cmdshell` 和 `FROM PROGRAM` 会被拒绝。

## 迁移

| 路径 | 说明 |
| --- | --- |
| `POST /api/migration/create` | `connection_id`、`version_no`、`name`、`up_sql`、`down_sql` |
| `POST /api/migration/update` | 只能改草稿或失败记录，改完回到草稿 |
| `POST /api/migration/one` | 单条，含 SQL |
| `POST /api/migration/search` | 按连接分页 |
| `POST /api/migration/delete` | 只能删除草稿或失败记录 |
| `POST /api/migration/apply` | 执行升级。可指定 `id`，否则按版本执行该连接的全部草稿 |
| `POST /api/migration/rollback` | 只回滚该连接最新的已执行版本，且必须有回滚 SQL |
| `POST /api/migration/resolve` | `action` 为 `abort` 或 `confirm`，处理中断后的 `applying` / `reverting` |

状态：`draft`、`applying`、`applied`、`failed`、`reverting`。

## 备份

| 路径 | 说明 |
| --- | --- |
| `POST /api/backup/create` | `format` 为 `sql` 或 `sqlite`。`sqlite` 只适用于 SQLite 连接 |
| `POST /api/backup/one` | 单条 |
| `POST /api/backup/search` | 分页 |
| `POST /api/backup/delete` | 删除记录和文件。进行中的备份不能删 |
| `POST /api/backup/restore` | 只恢复就绪的 SQL 备份 |
| `GET /api/backup/download?id=` | 下载文件。仍需要登录 Cookie 和 `X-DBGo-Request` |

SQL 备份最多 200 张表，并受 `backup.max_rows` 与 `backup.max_bytes` 限制。文件放在数据目录的 `backups/` 下，路径不能逃出该目录。

## 健康

| 路径 | HTTP |
| --- | --- |
| `GET /health/live` | 200 |
| `GET /health/ready` | 200 或 503 |
