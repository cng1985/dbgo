package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cng1985/dbgo/internal/platform/logging"
	"github.com/cng1985/dbgo/internal/platform/sqlscript"
	"github.com/cng1985/dbgo/migrations"
)

type scriptFile struct {
	Version  int
	Name     string
	Checksum string
	SQL      string
}

// Migrate 在专用连接上互斥执行嵌入的版本脚本。失败时启动中止。
func Migrate(ctx context.Context, handle *Handle, log *logging.Logger) (err error) {
	if handle == nil {
		return fmt.Errorf("数据库未初始化")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	conn, err := handle.SQL.Conn(ctx)
	if err != nil {
		return fmt.Errorf("获取迁移连接失败: %w", err)
	}
	defer conn.Close()
	if err = acquireLock(ctx, conn, handle.Driver); err != nil {
		return err
	}
	defer func() {
		releaseLock(ctx, conn, handle.Driver, err == nil)
	}()

	if err := ensureBookkeeping(ctx, conn, handle.Driver); err != nil {
		return err
	}
	files, err := loadScripts(handle.Driver)
	if err != nil {
		return err
	}
	applied, err := loadApplied(ctx, conn)
	if err != nil {
		return err
	}
	for _, file := range files {
		if checksum, ok := applied[file.Version]; ok {
			if checksum != file.Checksum {
				return fmt.Errorf("迁移 %d 已执行但内容已变化", file.Version)
			}
			continue
		}
		log.Info("执行数据库迁移", "version", file.Version, "name", file.Name, "driver", handle.Driver)
		if err := execScript(ctx, conn, file.SQL); err != nil {
			return fmt.Errorf("迁移 %d 失败: %w", file.Version, err)
		}
		if err := execArgs(ctx, conn, handle.Driver,
			"insert into schema_migration(version, name, checksum, applied_at) values (?, ?, ?, ?)",
			file.Version, file.Name, file.Checksum, time.Now().UTC()); err != nil {
			return fmt.Errorf("记录迁移 %d 失败: %w", file.Version, err)
		}
	}
	return nil
}

func loadScripts(driver string) ([]scriptFile, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("读取迁移目录失败: %w", err)
	}
	suffix := "." + driver + ".sql"
	var files []scriptFile
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, suffix) {
			continue
		}
		versionText, rest, ok := strings.Cut(name, "_")
		if !ok {
			return nil, fmt.Errorf("迁移文件名无效: %s", name)
		}
		version, err := strconv.Atoi(versionText)
		if err != nil || version < 1 {
			return nil, fmt.Errorf("迁移版本无效: %s", name)
		}
		raw, err := migrations.FS.ReadFile(name)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(raw)
		logical := strings.TrimSuffix(rest, suffix)
		files = append(files, scriptFile{
			Version:  version,
			Name:     logical,
			Checksum: hex.EncodeToString(sum[:]),
			SQL:      string(raw),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Version < files[j].Version })
	if len(files) == 0 {
		return nil, fmt.Errorf("没有适用于 %s 的迁移脚本", driver)
	}
	return files, nil
}

func loadApplied(ctx context.Context, conn *sql.Conn) (map[int]string, error) {
	rows, err := conn.QueryContext(ctx, "select version, checksum from schema_migration")
	if err != nil {
		return nil, fmt.Errorf("读取迁移记录失败: %w", err)
	}
	defer rows.Close()
	applied := map[int]string{}
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, err
		}
		applied[version] = strings.TrimSpace(checksum)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return applied, nil
}

func execScript(ctx context.Context, conn *sql.Conn, script string) error {
	for _, statement := range sqlscript.Split(script) {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func ensureBookkeeping(ctx context.Context, conn *sql.Conn, driver string) error {
	statements := bookkeepingSQL(driver)
	for _, statement := range statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("初始化迁移记录表失败: %w", err)
		}
	}
	return nil
}

func bookkeepingSQL(driver string) []string {
	switch driver {
	case "mysql":
		return []string{
			`create table if not exists schema_migration (
				version bigint not null primary key,
				name varchar(200) not null,
				checksum varchar(64) not null,
				applied_at datetime(3) not null
			) engine=innodb default charset=utf8mb4`,
			`create table if not exists migration_lock (
				id int not null primary key,
				note varchar(40) not null
			) engine=innodb default charset=utf8mb4`,
			`insert ignore into migration_lock(id, note) values (1, 'dbgo')`,
		}
	case "postgres":
		return []string{
			`create table if not exists schema_migration (
				version bigint primary key,
				name varchar(200) not null,
				checksum varchar(64) not null,
				applied_at timestamptz not null
			)`,
			`create table if not exists migration_lock (
				id integer primary key,
				note varchar(40) not null
			)`,
			`insert into migration_lock(id, note) values (1, 'dbgo') on conflict (id) do nothing`,
		}
	case "sqlserver":
		return []string{
			`if object_id(N'schema_migration', N'U') is null
			 create table schema_migration (
				version bigint not null primary key,
				name nvarchar(200) not null,
				checksum nvarchar(64) not null,
				applied_at datetimeoffset not null
			 )`,
			`if object_id(N'migration_lock', N'U') is null
			 create table migration_lock (
				id int not null primary key,
				note nvarchar(40) not null
			 )`,
			`if not exists (select 1 from migration_lock where id = 1)
			 insert into migration_lock(id, note) values (1, N'dbgo')`,
		}
	default:
		return []string{
			`create table if not exists schema_migration (
				version integer primary key,
				name text not null,
				checksum text not null,
				applied_at datetime not null
			)`,
			`create table if not exists migration_lock (
				id integer primary key,
				note text not null
			)`,
			`insert into migration_lock(id, note) select 1, 'dbgo' where not exists (select 1 from migration_lock where id = 1)`,
		}
	}
}

func acquireLock(ctx context.Context, conn *sql.Conn, driver string) error {
	switch driver {
	case "mysql":
		var ok sql.NullInt64
		if err := conn.QueryRowContext(ctx, "select get_lock('dbgo_migrate', 30)").Scan(&ok); err != nil {
			return fmt.Errorf("获取迁移锁失败: %w", err)
		}
		if !ok.Valid || ok.Int64 != 1 {
			return fmt.Errorf("获取迁移锁失败")
		}
	case "postgres":
		if _, err := conn.ExecContext(ctx, "select pg_advisory_lock(8642001)"); err != nil {
			return fmt.Errorf("获取迁移锁失败: %w", err)
		}
	case "sqlserver":
		var result sql.NullInt64
		err := conn.QueryRowContext(ctx,
			"declare @result int; exec @result = sp_getapplock @Resource = N'dbgo_migrate', @LockMode = N'Exclusive', @LockOwner = N'Session', @LockTimeout = 30000; select @result").Scan(&result)
		if err != nil {
			return fmt.Errorf("获取迁移锁失败: %w", err)
		}
		if !result.Valid || result.Int64 < 0 {
			return fmt.Errorf("获取迁移锁失败")
		}
	default:
		if _, err := conn.ExecContext(ctx, "begin immediate"); err != nil {
			return fmt.Errorf("获取 sqlite 迁移锁失败: %w", err)
		}
	}
	return nil
}

func releaseLock(ctx context.Context, conn *sql.Conn, driver string, success bool) {
	switch driver {
	case "mysql":
		_, _ = conn.ExecContext(ctx, "select release_lock('dbgo_migrate')")
	case "postgres":
		_, _ = conn.ExecContext(ctx, "select pg_advisory_unlock(8642001)")
	case "sqlserver":
		_, _ = conn.ExecContext(ctx, "exec sp_releaseapplock @Resource = N'dbgo_migrate', @LockOwner = N'Session'")
	default:
		statement := "rollback"
		if success {
			statement = "commit"
		}
		_, _ = conn.ExecContext(ctx, statement)
	}
}

func execArgs(ctx context.Context, conn *sql.Conn, driver, query string, args ...any) error {
	_, err := conn.ExecContext(ctx, rewritePlaceholders(driver, query), args...)
	return err
}

func rewritePlaceholders(driver, query string) string {
	if driver != "postgres" {
		return query
	}
	var builder strings.Builder
	number := 1
	inQuote := false
	for _, ch := range query {
		if ch == '\'' {
			inQuote = !inQuote
		}
		if ch == '?' && !inQuote {
			builder.WriteByte('$')
			builder.WriteString(strconv.Itoa(number))
			number++
			continue
		}
		builder.WriteRune(ch)
	}
	return builder.String()
}
