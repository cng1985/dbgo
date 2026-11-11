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
| 10005 | 大模型 |
| 10006 | 性能监控 |

主键字段是字符串。时间是 RFC3339。

## 认证

| 路径 | 说明 |
| --- | --- |
| `POST /api/auth/login` | 公开。成功后写 Cookie，响应不包含会话令牌 |
| `POST /api/auth/register` | 公开。`username`、`password`、`display_name`、`email`。成功后写 Cookie，角色为 `member`。未开放注册时返回 403，用户名重复返回 409 |
| `POST /api/auth/options` | 公开。返回 `allow_register` |
| `POST /api/auth/logout` | 撤销当前会话并清除 Cookie |
| `POST /api/auth/profile` | 当前用户，含邮箱和创建时间 |
| `POST /api/auth/profile/update` | `display_name`、`email` |
| `POST /api/auth/password` | `current_password`、`new_password`。成功后所有会话失效 |

## 总览与连接

| 路径 | 说明 |
| --- | --- |
| `POST /api/overview/one` | 数量和最近 5 条查询 |
| `POST /api/connection/create` | 新建。客户端不能指定 id |
| `POST /api/connection/update` | 全量更新，必须带 `id` 和 `version`。密码空字符串表示保持原值，备注空字符串会清空 |
| `POST /api/connection/one` | 单条 |
| `POST /api/connection/search` | 分页，只返回当前用户创建的连接。`keyword`、`driver`、`sort`、`direction` |
| `POST /api/connection/delete` | 不存在返回 404。重复删除仍是 404 |
| `POST /api/connection/test` | 长请求。返回版本和耗时 |
| `POST /api/connection/export` | 导出当前用户的连接。`ids` 为空时导出全部，超过 500 个要分批。`include_password` 为 true 时文件包含口令，默认不包含 |
| `POST /api/connection/import` | `document` 为 `dbgo-connections` 版本 1。`on_conflict` 为 `skip`、`rename` 或 `error`，默认改名。一次最多 200 个 |

SQLite 的 `sqlite_path` 可以是相对数据目录的路径。网络库需要主机、库名和用户名，端口 0 使用默认端口。PostgreSQL 的 `ssl_mode` 可以是 `disable`、`require`、`verify-ca`、`verify-full`。

## 查询与目录

| 路径 | 说明 |
| --- | --- |
| `POST /api/query/execute` | `connection_id`、`sql`、`read_only`、`max_rows`、`timeout_seconds`。`read_only` 省略时为 true |
| `POST /api/query/one` | 历史详情，不含结果网格 |
| `POST /api/query/search` | 可按连接和 `success`/`failed` 筛选 |
| `POST /api/catalog/schemas` | 模式 |
| `POST /api/catalog/tables` | 表 |
| `POST /api/catalog/columns` | 字段。类型带长度或精度，并返回 `comment`、`identity` |
| `POST /api/catalog/indexes` | 索引 |
| `POST /api/catalog/suggest` | 查询提示。返回驱动、当前模式、模式列表，以及该模式下的表和字段。省略 `schema` 时用连接的默认模式。最多 300 张表、6000 个字段，超出时 `truncated` 为 true。SQL Server 使用 `@p1` 参数 |
| `POST /api/catalog/objects` | `kind` 为 `table`、`view`、`procedure`、`function`、`routine` 或空。SQLite 的例程 `supported` 为 false |
| `POST /api/catalog/definition` | 读取表、视图、存储过程或函数的 SQL。表还会返回全部字段，含类型、默认值、自增和注释 |
| `POST /api/catalog/preview` | 按列、查询或例程主体生成 DDL，不连接目标库。列可以带 `comment`。页面建表时按驱动选择列类型，已有类型不在预设里时仍保留。`mode` 为 `alter` 时按 `baseline` 和当前列生成更新表结构的语句，不执行；`changed` 为 true 才表示有语句。列的 `origin` 是修改前的字段名，空字符串表示新增。省略全部 `origin` 时按字段名对应 |
| `POST /api/catalog/apply` | 长请求。在选定连接上执行最多 300 条对象语句。SQLite 在同一条连接的事务里执行，执行期间关闭外键强制，提交前若出现新的外键错误则整段回滚 |

单次查询最多 50 条语句。结果行数默认 100，不超过配置上限。`load_extension`、`xp_cmdshell` 和 `FROM PROGRAM` 会被拒绝。只读模式允许查询语句，也允许 SQL Server 的 `PRINT`。成功时每条语句的 `messages` 是 `PRINT` 和信息类消息，查询页的输出控制台显示这些文本。每条语句同时带回执行的 `sql`，查询页用它判断单表结果能否编辑行。

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

## 大模型与性能

| 路径 | 说明 |
| --- | --- |
| `POST /api/llm/create` | `name`、`base_url`、`api_key`、`model`、`temperature`、`max_tokens`、`is_default`。地址只接受 http 或 https |
| `POST /api/llm/update` | 全量更新。`api_key` 为空表示保持原密钥 |
| `POST /api/llm/one` | 单条。响应只有 `api_key_set`，不返回密钥 |
| `POST /api/llm/search` | 分页 |
| `POST /api/llm/delete` | 删除配置 |
| `POST /api/llm/test` | 长请求。先请求 `/models`，失败再发一条短对话 |
| `POST /api/llm/optimize` | 长请求。`connection_id`、`sql`，可选 `profile_id` 和 `schema`。返回 `summary`、`sql`、`notes` |
| `POST /api/monitor/snapshot` | 长请求。返回指标、会话和对象大小。读不到的部分写入 `notes` |

模型服务走 OpenAI Chat Completions。密钥用 AES-GCM 保存。地址不能带账号，也不能指向云元数据地址。

## 代码段

| 路径 | 说明 |
| --- | --- |
| `POST /api/snippet/create` | `name`、`body`，可选 `description` 和 `driver`。每个账号最多 500 个，名称在自己的代码段里唯一 |
| `POST /api/snippet/update` | 全量更新，必须带 `id` 和 `version` |
| `POST /api/snippet/one` | 单条，含内容。看不到别人的代码段 |
| `POST /api/snippet/search` | 分页。列表不含内容 |
| `POST /api/snippet/delete` | 删除 |
| `POST /api/snippet/export` | `ids` 为空时导出全部。文件格式是 `dbgo-snippets` 版本 1 |
| `POST /api/snippet/import` | `document` 与 `on_conflict`。策略同连接导入，一次最多 500 个，与导出上限一致 |

`driver` 为空表示不限数据库，否则是 `sqlite`、`mysql`、`postgres` 或 `sqlserver`。

## 健康

| 路径 | HTTP |
| --- | --- |
| `GET /health/live` | 200 |
| `GET /health/ready` | 200 或 503 |
