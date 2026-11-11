package shared

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cng1985/dbgo/internal/platform/sqlscript"
)

func TestRenderTableAlterNoChange(t *testing.T) {
	columns := []ColumnDraft{{Name: "id", Type: "integer", Primary: true, Identity: true}}
	sqlText, changed, note, err := RenderTableAlter(TableAlter{
		Driver: "sqlite", BaselineName: "city", Name: "city",
		Baseline: columns, Columns: columns,
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed || sqlText != "" || note != "" {
		t.Fatalf("没有改动时不应生成语句: changed=%v note=%s sql=%s", changed, note, sqlText)
	}
}

func TestRenderTableAlterRejectsUnsafeColumn(t *testing.T) {
	_, _, _, err := RenderTableAlter(TableAlter{
		Driver: "mysql", BaselineName: "city", Name: "city",
		Baseline: []ColumnDraft{{Name: "id", Type: "int", Primary: true}},
		Columns: []ColumnDraft{
			{Name: "id", Type: "int", Primary: true},
			{Name: "name", Type: "varchar(40)", Nullable: false},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "默认值") {
		t.Fatalf("新增非空字段应要求默认值: %v", err)
	}
}

func TestRenderTableAlterSQLiteAddRenameAndRebuild(t *testing.T) {
	base := []ColumnDraft{
		{Name: "id", Type: "integer", Primary: true, Identity: true},
		{Name: "name", Type: "text", Nullable: false, Comment: "地名"},
		{Name: "code", Type: "text", Nullable: false, Default: "'CN'"},
	}
	added, changed, _, err := RenderTableAlter(TableAlter{
		Driver: "sqlite", BaselineName: "city", Name: "city",
		Baseline: base,
		Columns:  append(append([]ColumnDraft{}, base...), ColumnDraft{Name: "note", Type: "text", Nullable: true}),
	})
	if err != nil || !changed || added != `ALTER TABLE "city" ADD COLUMN "note" text` {
		t.Fatalf("新增可空字段应直接添加: %v %s", err, added)
	}
	renamed, _, _, err := RenderTableAlter(TableAlter{
		Driver: "sqlite", BaselineName: "city", Name: "city",
		Baseline: base,
		Columns: []ColumnDraft{
			base[0],
			base[1],
			{Origin: "code", Name: "region", Type: "text", Nullable: false, Default: "'CN'"},
		},
	})
	if err != nil || renamed != `ALTER TABLE "city" RENAME COLUMN "code" TO "region"` {
		t.Fatalf("重命名应生成 RENAME COLUMN: %v %s", err, renamed)
	}
	rebuilt, _, note, err := RenderTableAlter(TableAlter{
		Driver: "sqlite", BaselineName: "city", Name: "city",
		Baseline: base,
		Columns: []ColumnDraft{
			base[0],
			{Origin: "name", Name: "name", Type: "text", Nullable: false, Comment: "城市"},
			base[2],
		},
	})
	if err != nil || !strings.Contains(rebuilt, "INSERT INTO") || !strings.Contains(rebuilt, "城市") || !strings.Contains(note, "重建") {
		t.Fatalf("修改注释应重建表: %v %s %s", err, note, rebuilt)
	}
	if strings.Contains(rebuilt, "ADD COLUMN") {
		t.Fatalf("重建不应再混入 ADD COLUMN: %s", rebuilt)
	}
	dropped, _, dropNote, err := RenderTableAlter(TableAlter{
		Driver: "sqlite", BaselineName: "city", Name: "city",
		Baseline: base,
		Columns:  []ColumnDraft{base[0], base[1]},
	})
	if err != nil || dropped != `ALTER TABLE "city" DROP COLUMN "code"` || !strings.Contains(dropNote, "丢掉") {
		t.Fatalf("删除字段应生成 DROP COLUMN: %v %s %s", err, dropNote, dropped)
	}
	renamedTable, _, _, err := RenderTableAlter(TableAlter{
		Driver: "sqlite", BaselineName: "city", Name: "town",
		Baseline: base, Columns: base,
	})
	if err != nil || renamedTable != `ALTER TABLE "city" RENAME TO "town"` {
		t.Fatalf("应生成表重命名: %v %s", err, renamedTable)
	}
	temporal, _, _, err := RenderTableAlter(TableAlter{
		Driver: "sqlite", BaselineName: "city", Name: "city",
		Baseline: []ColumnDraft{{Name: "id", Type: "integer", Primary: true}},
		Columns: []ColumnDraft{
			{Name: "id", Type: "integer", Primary: true},
			{Name: "seen", Type: "text", Nullable: true, Default: "CURRENT_TIMESTAMP"},
		},
	})
	if err != nil || strings.Contains(temporal, "ADD COLUMN") || !strings.Contains(temporal, "INSERT INTO") {
		t.Fatalf("可空的时间默认值也应重建表: %v %s", err, temporal)
	}
}

func TestRenderTableAlterSwapsNames(t *testing.T) {
	sqlText, _, _, err := RenderTableAlter(TableAlter{
		Driver: "sqlite", BaselineName: "city", Name: "city",
		Baseline: []ColumnDraft{
			{Name: "id", Type: "integer", Primary: true},
			{Name: "left_name", Type: "text", Nullable: true},
			{Name: "right_name", Type: "text", Nullable: true},
		},
		Columns: []ColumnDraft{
			{Name: "id", Type: "integer", Primary: true},
			{Origin: "left_name", Name: "right_name", Type: "text", Nullable: true},
			{Origin: "right_name", Name: "left_name", Type: "text", Nullable: true},
		},
	})
	if err != nil || !strings.Contains(sqlText, "__dbgo_col_1") || strings.Count(sqlText, "RENAME COLUMN") != 3 {
		t.Fatalf("对调列名应经过临时列: %v %s", err, sqlText)
	}
}

func TestRenderTableAlterOtherDrivers(t *testing.T) {
	base := []ColumnDraft{{Name: "id", Type: "int", Primary: true}, {Name: "name", Type: "text", Nullable: true}}
	mysql, _, _, err := RenderTableAlter(TableAlter{
		Driver: "mysql", Schema: "app", BaselineSchema: "app", BaselineName: "city", Name: "city",
		Baseline: base,
		Columns: []ColumnDraft{
			base[0],
			{Origin: "name", Name: "name", Type: "varchar(40)", Nullable: false, Comment: "城市"},
		},
	})
	if err != nil || !strings.Contains(mysql, "MODIFY COLUMN") || !strings.Contains(mysql, "COMMENT '城市'") || !strings.Contains(mysql, "NOT NULL") {
		t.Fatalf("MySQL 应修改字段和注释: %v %s", err, mysql)
	}
	postgres, _, _, err := RenderTableAlter(TableAlter{
		Driver: "postgres", Schema: "public", BaselineSchema: "public", BaselineName: "city", Name: "town",
		Baseline: base,
		Columns: []ColumnDraft{
			{Origin: "id", Name: "id", Type: "bigint", Primary: true},
			{Origin: "name", Name: "title", Type: "text", Nullable: true, Comment: "标题"},
		},
	})
	if err != nil || !strings.Contains(postgres, `TYPE bigint USING "id"::bigint`) || !strings.Contains(postgres, `RENAME COLUMN "name" TO "title"`) || !strings.Contains(postgres, `COMMENT ON COLUMN "public"."city"."title" IS '标题'`) || !strings.Contains(postgres, `RENAME TO "town"`) {
		t.Fatalf("PostgreSQL 更新语句不完整: %v %s", err, postgres)
	}
	parts := sqlscript.Split(postgres)
	for _, part := range parts {
		if strings.Contains(part, "DO $dbgo$") && strings.Count(part, ";") < 2 {
			t.Fatalf("删除主键的 DO 块不应被拆开: %#v", parts)
		}
	}
	server, _, note, err := RenderTableAlter(TableAlter{
		Driver: "mssql", BaselineName: "city", Name: "city",
		Baseline: []ColumnDraft{{Name: "id", Type: "int", Nullable: false}, {Name: "name", Type: "nvarchar(20)", Nullable: true, Comment: "旧"}},
		Columns: []ColumnDraft{
			{Origin: "id", Name: "id", Type: "int", Nullable: false, Identity: true, Primary: true},
			{Origin: "name", Name: "name", Type: "nvarchar(20)", Nullable: true, Comment: "新"},
		},
	})
	if err != nil || !strings.Contains(server, "IDENTITY_INSERT") || !strings.Contains(server, "sp_rename") || !strings.Contains(note, "自增") {
		t.Fatalf("SQL Server 修改自增应重建: %v %s %s", err, note, server)
	}
	comment, _, _, err := RenderTableAlter(TableAlter{
		Driver: "sqlserver", Schema: "dbo", BaselineSchema: "dbo", BaselineName: "city", Name: "city",
		Baseline: []ColumnDraft{{Name: "id", Type: "int", Primary: true}, {Name: "name", Type: "nvarchar(20)", Nullable: true}},
		Columns: []ColumnDraft{
			{Name: "id", Type: "int", Primary: true},
			{Origin: "name", Name: "name", Type: "nvarchar(20)", Nullable: true, Comment: "城市"},
		},
	})
	if err != nil || !strings.Contains(comment, "sp_addextendedproperty") {
		t.Fatalf("SQL Server 应写入字段注释: %v %s", err, comment)
	}
	blocks := sqlscript.Split(comment)
	found := false
	for _, block := range blocks {
		if strings.Contains(block, "BEGIN") && strings.Contains(block, "sp_addextendedproperty") && strings.Contains(block, "END") {
			found = true
		}
	}
	if !found {
		t.Fatalf("注释批次应保持为一条语句: %#v", blocks)
	}
}

func TestSQLiteAlterKeepsRows(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "alter.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `create table city (
		id integer primary key autoincrement,
		name text not null /* 地名 */,
		code text not null default 'CN'
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `insert into city (name, code) values ('杭州', 'HZ')`); err != nil {
		t.Fatal(err)
	}
	base := []ColumnDraft{
		{Name: "id", Type: "integer", Primary: true, Identity: true},
		{Name: "name", Type: "text", Nullable: false, Comment: "地名"},
		{Name: "code", Type: "text", Nullable: false, Default: "'CN'"},
	}
	renamed := []ColumnDraft{
		{Origin: "id", Name: "id", Type: "integer", Primary: true, Identity: true},
		{Origin: "name", Name: "name", Type: "text", Nullable: false, Comment: "地名"},
		{Origin: "code", Name: "region", Type: "text", Nullable: false, Default: "'CN'"},
	}
	if err := applyAlter(ctx, db, TableAlter{Driver: "sqlite", BaselineName: "city", Name: "city", Baseline: base, Columns: renamed}); err != nil {
		t.Fatal(err)
	}
	var region string
	if err := db.QueryRowContext(ctx, `select region from city`).Scan(&region); err != nil || region != "HZ" {
		t.Fatalf("重命名后应保留原值: %v %s", err, region)
	}
	current := []ColumnDraft{
		{Name: "id", Type: "integer", Primary: true, Identity: true},
		{Name: "name", Type: "text", Nullable: false, Comment: "地名"},
		{Name: "region", Type: "text", Nullable: false, Default: "'CN'"},
	}
	withNote := append(append([]ColumnDraft{}, current...), ColumnDraft{Name: "note", Type: "text", Nullable: true})
	if err := applyAlter(ctx, db, TableAlter{Driver: "sqlite", BaselineName: "city", Name: "city", Baseline: current, Columns: withNote}); err != nil {
		t.Fatal(err)
	}
	rebuilt := []ColumnDraft{
		current[0],
		{Name: "name", Type: "text", Nullable: false, Comment: "城市"},
		current[2],
		{Name: "note", Type: "text", Nullable: true},
	}
	if err := applyAlter(ctx, db, TableAlter{Driver: "sqlite", BaselineName: "city", Name: "city", Baseline: withNote, Columns: rebuilt}); err != nil {
		t.Fatal(err)
	}
	items, err := ListColumns(ctx, db, "sqlite", "main", "city")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]ColumnInfo{}
	for _, item := range items {
		byName[item.Name] = item
	}
	if len(items) != 4 || byName["name"].Comment != "城市" || byName["region"].Name != "region" || byName["id"].Identity != true {
		t.Fatalf("重建后的字段不正确: %#v", items)
	}
	var name string
	if err := db.QueryRowContext(ctx, `select name from city`).Scan(&name); err != nil || name != "杭州" {
		t.Fatalf("重建后应保留数据: %v %s", err, name)
	}
}

func applyAlter(ctx context.Context, db *sql.DB, spec TableAlter) error {
	sqlText, changed, _, err := RenderTableAlter(spec)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	_, err = Execute(ctx, db, "sqlite", sqlText, ExecOptions{MaxRows: 10, MaxStatements: 30, SQLiteSchema: true})
	return err
}

func TestSQLiteSchemaRollsBackFailedRebuild(t *testing.T) {
	db := openSQLite(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `create table city (id integer primary key, name text)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `insert into city (id, name) values (1, null)`); err != nil {
		t.Fatal(err)
	}
	err := applyAlter(ctx, db, TableAlter{
		Driver: "sqlite", BaselineName: "city", Name: "city",
		Baseline: []ColumnDraft{
			{Name: "id", Type: "integer", Primary: true},
			{Name: "name", Type: "text", Nullable: true},
		},
		Columns: []ColumnDraft{
			{Name: "id", Type: "integer", Primary: true},
			{Name: "name", Type: "text", Nullable: false},
		},
	})
	if err == nil {
		t.Fatal("把含空值的列改成非空应失败")
	}
	var leftover int
	if err := db.QueryRowContext(ctx, `select count(*) from sqlite_master where name like '__dbgo_new%'`).Scan(&leftover); err != nil || leftover != 0 {
		t.Fatalf("失败后不应留下临时表: %v %d", err, leftover)
	}
	var name sql.NullString
	if err := db.QueryRowContext(ctx, `select name from city where id = 1`).Scan(&name); err != nil || name.Valid {
		t.Fatalf("失败后原表应保持原样: %v %#v", err, name)
	}
	if err := applyAlter(ctx, db, TableAlter{
		Driver: "sqlite", BaselineName: "city", Name: "city",
		Baseline: []ColumnDraft{
			{Name: "id", Type: "integer", Primary: true},
			{Name: "name", Type: "text", Nullable: true},
		},
		Columns: []ColumnDraft{
			{Name: "id", Type: "integer", Primary: true},
			{Name: "name", Type: "text", Nullable: true, Comment: "名称"},
		},
	}); err != nil {
		t.Fatalf("回滚后应能再次更新: %v", err)
	}
	if !sqliteForeignKeysOn(t, db) {
		t.Fatal("执行结束后应恢复外键强制")
	}
}

func TestSQLiteSchemaKeepsReferencingRows(t *testing.T) {
	db := openSQLite(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `create table parent (id integer primary key, name text not null);
		create table child (id integer primary key, parent_id integer not null references parent(id) on delete cascade);
		insert into parent (id, name) values (1, '杭州');
		insert into child (id, parent_id) values (1, 1)`); err != nil {
		t.Fatal(err)
	}
	if err := applyAlter(ctx, db, TableAlter{
		Driver: "sqlite", BaselineName: "parent", Name: "parent",
		Baseline: []ColumnDraft{
			{Name: "id", Type: "integer", Primary: true},
			{Name: "name", Type: "text", Nullable: false},
		},
		Columns: []ColumnDraft{
			{Name: "id", Type: "integer", Primary: true},
			{Name: "name", Type: "text", Nullable: false, Comment: "地名"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	var children int
	if err := db.QueryRowContext(ctx, `select count(*) from child`).Scan(&children); err != nil || children != 1 {
		t.Fatalf("重建父表不应级联删除子表: %v %d", err, children)
	}
	_, err := Execute(ctx, db, "sqlite", `drop table parent`, ExecOptions{MaxStatements: 5, SQLiteSchema: true})
	if err == nil || !strings.Contains(err.Error(), "外键") {
		t.Fatalf("破坏外键的语句应回滚: %v", err)
	}
	var parents int
	if err := db.QueryRowContext(ctx, `select count(*) from parent`).Scan(&parents); err != nil || parents != 1 {
		t.Fatalf("回滚后父表应仍在: %v %d", err, parents)
	}
	if err := db.QueryRowContext(ctx, `select count(*) from child`).Scan(&children); err != nil || children != 1 {
		t.Fatalf("回滚后子表行应仍在: %v %d", err, children)
	}
	if !sqliteForeignKeysOn(t, db) {
		t.Fatal("失败后也应恢复外键强制")
	}
}

func openSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "alter.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	return db
}

func sqliteForeignKeysOn(t *testing.T, db *sql.DB) bool {
	t.Helper()
	var flag int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&flag); err != nil {
		t.Fatal(err)
	}
	return flag == 1
}
