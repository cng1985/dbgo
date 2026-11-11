package shared

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

type Metric struct {
	Key   string
	Label string
	Value string
	Unit  string
	Hint  string
}

type Activity struct {
	ID       string
	User     string
	Database string
	State    string
	Duration string
	SQL      string
}

type RelationSize struct {
	Schema string
	Name   string
	Rows   int64
	Bytes  int64
}

type Snapshot struct {
	Metrics  []Metric
	Sessions []Activity
	Sizes    []RelationSize
	Notes    []string
}

func CollectSnapshot(ctx context.Context, db *sql.DB, driver, schema string) (Snapshot, error) {
	switch NormalizeDriver(driver) {
	case "sqlite":
		return collectSQLite(ctx, db)
	case "mysql":
		return collectMySQL(ctx, db, schema)
	case "postgres":
		return collectPostgres(ctx, db, schema)
	case "sqlserver":
		return collectSQLServer(ctx, db, schema)
	default:
		return Snapshot{}, fmt.Errorf("不支持的数据库类型")
	}
}

func collectSQLite(ctx context.Context, db *sql.DB) (Snapshot, error) {
	snap := emptySnapshot()
	pageCount, err := scalarInt(ctx, db, "pragma page_count")
	pageSize, sizeErr := scalarInt(ctx, db, "pragma page_size")
	freePages, freeErr := scalarInt(ctx, db, "pragma freelist_count")
	if err != nil || sizeErr != nil {
		return snap, fmt.Errorf("读取 SQLite 页信息失败")
	}
	snap.Metrics = append(snap.Metrics,
		Metric{Key: "database_bytes", Label: "库大小", Value: strconv.FormatInt(pageCount*pageSize, 10), Unit: "bytes"},
		Metric{Key: "page_size", Label: "页大小", Value: strconv.FormatInt(pageSize, 10), Unit: "bytes"},
		Metric{Key: "page_count", Label: "页数", Value: strconv.FormatInt(pageCount, 10)},
	)
	if freeErr == nil {
		snap.Metrics = append(snap.Metrics, Metric{Key: "free_pages", Label: "空闲页", Value: strconv.FormatInt(freePages, 10)})
	}
	snap.Notes = append(snap.Notes, "SQLite 是嵌入式库，没有独立的会话列表")
	rows, err := db.QueryContext(ctx, "select name from sqlite_master where type = 'table' and name not like 'sqlite_%' order by name limit 40")
	if err != nil {
		snap.Notes = append(snap.Notes, "读取表大小失败")
		return snap, nil
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return snap, err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return snap, err
	}
	for _, name := range names {
		quoted, err := QuoteIdent("sqlite", name)
		if err != nil {
			continue
		}
		count, err := scalarInt(ctx, db, "select count(*) from "+quoted)
		if err != nil {
			continue
		}
		snap.Sizes = append(snap.Sizes, RelationSize{Schema: "main", Name: name, Rows: count})
	}
	return snap, nil
}

func collectMySQL(ctx context.Context, db *sql.DB, schema string) (Snapshot, error) {
	snap := emptySnapshot()
	rows, err := db.QueryContext(ctx, `show global status where variable_name in (
		'Uptime','Threads_connected','Threads_running','Questions','Slow_queries',
		'Bytes_received','Bytes_sent','Innodb_buffer_pool_read_requests','Innodb_buffer_pool_reads')`)
	if err != nil {
		snap.Notes = append(snap.Notes, "读取状态变量失败："+clip(err.Error()))
	} else {
		status := map[string]string{}
		for rows.Next() {
			var key, value string
			if err := rows.Scan(&key, &value); err != nil {
				rows.Close()
				return snap, err
			}
			status[strings.ToLower(key)] = value
		}
		rows.Close()
		add := func(key, label, unit string) {
			if value, ok := status[strings.ToLower(key)]; ok {
				snap.Metrics = append(snap.Metrics, Metric{Key: strings.ToLower(key), Label: label, Value: value, Unit: unit})
			}
		}
		add("Uptime", "运行时间", "seconds")
		add("Threads_connected", "连接数", "")
		add("Threads_running", "活跃线程", "")
		add("Questions", "累计语句", "")
		add("Slow_queries", "慢查询", "")
		add("Bytes_received", "接收字节", "bytes")
		add("Bytes_sent", "发送字节", "bytes")
		if reads, ok1 := status["innodb_buffer_pool_reads"]; ok1 {
			requests, ok2 := status["innodb_buffer_pool_read_requests"]
			if ok2 {
				if hit := ratio(requests, reads); hit != "" {
					snap.Metrics = append(snap.Metrics, Metric{
						Key:   "buffer_pool_hit",
						Label: "缓冲池命中",
						Value: hit,
						Hint:  "1 减去磁盘读除以逻辑读",
					})
				}
			}
		}
	}
	activity, err := db.QueryContext(ctx, `select id, user, ifnull(db, ''), command, time, left(ifnull(info, ''), 500)
		from information_schema.processlist order by time desc limit 50`)
	if err != nil {
		snap.Notes = append(snap.Notes, "读取会话失败："+clip(err.Error()))
	} else {
		defer activity.Close()
		for activity.Next() {
			var item Activity
			var id int64
			var seconds int64
			if err := activity.Scan(&id, &item.User, &item.Database, &item.State, &seconds, &item.SQL); err != nil {
				return snap, err
			}
			item.ID = strconv.FormatInt(id, 10)
			item.Duration = strconv.FormatInt(seconds, 10) + "s"
			snap.Sessions = append(snap.Sessions, item)
		}
		if err := activity.Err(); err != nil {
			return snap, err
		}
	}
	if schema != "" {
		sizes, err := db.QueryContext(ctx, RewritePlaceholders("mysql", `select table_schema, table_name, ifnull(table_rows, 0), ifnull(data_length, 0) + ifnull(index_length, 0)
			from information_schema.tables
			where table_schema = ?
			order by ifnull(data_length, 0) + ifnull(index_length, 0) desc
			limit 30`), schema)
		if err != nil {
			snap.Notes = append(snap.Notes, "读取表大小失败："+clip(err.Error()))
		} else {
			defer sizes.Close()
			for sizes.Next() {
				var item RelationSize
				if err := sizes.Scan(&item.Schema, &item.Name, &item.Rows, &item.Bytes); err != nil {
					return snap, err
				}
				snap.Sizes = append(snap.Sizes, item)
			}
		}
	}
	return snap, nil
}

func collectPostgres(ctx context.Context, db *sql.DB, schema string) (Snapshot, error) {
	snap := emptySnapshot()
	uptime, err := scalarInt(ctx, db, "select extract(epoch from (now() - pg_postmaster_start_time()))::bigint")
	if err == nil {
		snap.Metrics = append(snap.Metrics, Metric{Key: "uptime", Label: "运行时间", Value: strconv.FormatInt(uptime, 10), Unit: "seconds"})
	} else {
		snap.Notes = append(snap.Notes, "读取运行时间失败："+clip(err.Error()))
	}
	var active, total int64
	err = db.QueryRowContext(ctx, "select count(*) filter (where state = 'active'), count(*) from pg_stat_activity").Scan(&active, &total)
	if err == nil {
		snap.Metrics = append(snap.Metrics,
			Metric{Key: "sessions", Label: "会话数", Value: strconv.FormatInt(total, 10)},
			Metric{Key: "active_sessions", Label: "活跃会话", Value: strconv.FormatInt(active, 10)},
		)
	} else {
		snap.Notes = append(snap.Notes, "读取会话统计失败："+clip(err.Error()))
	}
	size, err := scalarInt(ctx, db, "select pg_database_size(current_database())")
	if err == nil {
		snap.Metrics = append(snap.Metrics, Metric{Key: "database_bytes", Label: "库大小", Value: strconv.FormatInt(size, 10), Unit: "bytes"})
	}
	var hits, reads sql.NullInt64
	err = db.QueryRowContext(ctx, "select sum(blks_hit), sum(blks_read) from pg_stat_database where datname = current_database()").Scan(&hits, &reads)
	if err == nil && hits.Valid && reads.Valid && hits.Int64+reads.Int64 > 0 {
		snap.Metrics = append(snap.Metrics, Metric{
			Key:   "cache_hit",
			Label: "缓存命中率",
			Value: strconv.FormatFloat(float64(hits.Int64)/float64(hits.Int64+reads.Int64), 'f', 4, 64),
		})
	}
	activity, err := db.QueryContext(ctx, `select pid::text, coalesce(usename, ''), coalesce(datname, ''), coalesce(state, ''),
		coalesce(extract(epoch from (now() - query_start))::bigint::text, ''), left(coalesce(query, ''), 500)
		from pg_stat_activity
		where datname = current_database()
		order by query_start nulls last
		limit 50`)
	if err != nil {
		snap.Notes = append(snap.Notes, "读取活动会话失败："+clip(err.Error()))
	} else {
		defer activity.Close()
		for activity.Next() {
			var item Activity
			if err := activity.Scan(&item.ID, &item.User, &item.Database, &item.State, &item.Duration, &item.SQL); err != nil {
				return snap, err
			}
			if item.Duration != "" {
				item.Duration += "s"
			}
			snap.Sessions = append(snap.Sessions, item)
		}
	}
	sizeQuery := `select schemaname, relname, coalesce(n_live_tup, 0), pg_total_relation_size(relid)
		from pg_stat_user_tables`
	args := []any{}
	if schema != "" {
		sizeQuery += " where schemaname = $1"
		args = append(args, schema)
	}
	sizeQuery += " order by pg_total_relation_size(relid) desc limit 30"
	sizes, err := db.QueryContext(ctx, sizeQuery, args...)
	if err != nil {
		snap.Notes = append(snap.Notes, "读取表大小失败："+clip(err.Error()))
	} else {
		defer sizes.Close()
		for sizes.Next() {
			var item RelationSize
			if err := sizes.Scan(&item.Schema, &item.Name, &item.Rows, &item.Bytes); err != nil {
				return snap, err
			}
			snap.Sizes = append(snap.Sizes, item)
		}
	}
	return snap, nil
}

func collectSQLServer(ctx context.Context, db *sql.DB, schema string) (Snapshot, error) {
	snap := emptySnapshot()
	uptime, err := scalarInt(ctx, db, "select datediff(second, sqlserver_start_time, sysdatetimeoffset()) from sys.dm_os_sys_info")
	if err == nil {
		snap.Metrics = append(snap.Metrics, Metric{Key: "uptime", Label: "运行时间", Value: strconv.FormatInt(uptime, 10), Unit: "seconds"})
	} else {
		snap.Notes = append(snap.Notes, "读取运行时间需要 VIEW SERVER STATE："+clip(err.Error()))
	}
	var total int64
	err = db.QueryRowContext(ctx, "select count(*) from sys.dm_exec_sessions where is_user_process = 1").Scan(&total)
	if err == nil {
		snap.Metrics = append(snap.Metrics, Metric{Key: "sessions", Label: "用户会话", Value: strconv.FormatInt(total, 10)})
	} else {
		snap.Notes = append(snap.Notes, "读取会话统计失败："+clip(err.Error()))
	}
	activity, err := db.QueryContext(ctx, `select top 50 cast(s.session_id as varchar(20)), s.login_name, db_name(s.database_id), s.status,
		cast(datediff(second, s.last_request_start_time, sysdatetimeoffset()) as varchar(20)), left(coalesce(t.text, ''), 500)
		from sys.dm_exec_sessions s
		left join sys.dm_exec_requests r on r.session_id = s.session_id
		outer apply sys.dm_exec_sql_text(r.sql_handle) t
		where s.is_user_process = 1
		order by s.last_request_start_time desc`)
	if err != nil {
		snap.Notes = append(snap.Notes, "读取活动会话失败："+clip(err.Error()))
	} else {
		defer activity.Close()
		for activity.Next() {
			var item Activity
			var databaseName sql.NullString
			if err := activity.Scan(&item.ID, &item.User, &databaseName, &item.State, &item.Duration, &item.SQL); err != nil {
				return snap, err
			}
			item.Database = databaseName.String
			if item.Duration != "" {
				item.Duration += "s"
			}
			snap.Sessions = append(snap.Sessions, item)
		}
	}
	sizeQuery := `select top 30 s.name, t.name, sum(p.rows), sum(a.total_pages) * 8192
		from sys.tables t
		join sys.schemas s on s.schema_id = t.schema_id
		join sys.partitions p on p.object_id = t.object_id and p.index_id in (0, 1)
		join sys.allocation_units a on a.container_id = p.partition_id`
	args := []any{}
	if schema != "" {
		sizeQuery += " where s.name = ?"
		args = append(args, schema)
	}
	sizeQuery += " group by s.name, t.name order by sum(a.total_pages) desc"
	sizes, err := db.QueryContext(ctx, RewritePlaceholders("sqlserver", sizeQuery), args...)
	if err != nil {
		snap.Notes = append(snap.Notes, "读取表大小失败："+clip(err.Error()))
	} else {
		defer sizes.Close()
		for sizes.Next() {
			var item RelationSize
			if err := sizes.Scan(&item.Schema, &item.Name, &item.Rows, &item.Bytes); err != nil {
				return snap, err
			}
			snap.Sizes = append(snap.Sizes, item)
		}
	}
	return snap, nil
}

func emptySnapshot() Snapshot {
	return Snapshot{Metrics: []Metric{}, Sessions: []Activity{}, Sizes: []RelationSize{}, Notes: []string{}}
}

func scalarInt(ctx context.Context, db *sql.DB, query string, args ...any) (int64, error) {
	var value sql.NullInt64
	err := db.QueryRowContext(ctx, query, args...).Scan(&value)
	if err != nil {
		return 0, err
	}
	if !value.Valid {
		return 0, fmt.Errorf("没有返回数值")
	}
	return value.Int64, nil
}

func ratio(requests, diskReads string) string {
	req, err1 := strconv.ParseFloat(requests, 64)
	disk, err2 := strconv.ParseFloat(diskReads, 64)
	if err1 != nil || err2 != nil || req <= 0 {
		return ""
	}
	hit := 1 - disk/req
	if hit < 0 {
		hit = 0
	}
	return strconv.FormatFloat(hit, 'f', 4, 64)
}

func clip(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 180 {
		return value[:180]
	}
	return value
}
