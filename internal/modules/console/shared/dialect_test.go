package shared

import "testing"

func TestDriverAliasesAndIdentifiers(t *testing.T) {
	if NormalizeDriver("pgsql") != "postgres" || NormalizeDriver("mssql") != "sqlserver" || NormalizeDriver("oracle") != "" {
		t.Fatal("驱动别名不符合预期")
	}
	mysqlName, err := QuoteIdent("mysql", "order`line")
	if err != nil || mysqlName != "`order``line`" {
		t.Fatalf("mysql 标识符: %s %v", mysqlName, err)
	}
	serverName, err := QuoteIdent("sqlserver", "order]line")
	if err != nil || serverName != "[order]]line]" {
		t.Fatalf("sqlserver 标识符: %s %v", serverName, err)
	}
	pgName, err := QuoteIdent("postgres", `order"line`)
	if err != nil || pgName != `"order""line"` {
		t.Fatalf("postgres 标识符: %s %v", pgName, err)
	}
	if _, err := QuoteIdent("sqlite", ""); err == nil {
		t.Fatal("空标识符应被拒绝")
	}
}

func TestPostgresPlaceholdersStayInsideLiterals(t *testing.T) {
	got := RewritePlaceholders("postgres", "select '?' as mark where name = ? and note = '?'")
	want := "select '?' as mark where name = $1 and note = '?'"
	if got != want {
		t.Fatalf("占位符改写错误: %s", got)
	}
	server := RewritePlaceholders("mssql", "select '?' as mark where name = ? and note = '?' and id = ?")
	wantServer := "select '?' as mark where name = @p1 and note = '?' and id = @p2"
	if server != wantServer {
		t.Fatalf("sqlserver 占位符改写错误: %s", server)
	}
	if RewritePlaceholders("mysql", "select ?") != "select ?" {
		t.Fatal("mysql 不应改写占位符")
	}
}

func TestForbiddenStatements(t *testing.T) {
	if !Forbidden("select load_extension('x')") || !Forbidden("xp_cmdshell 'dir'") || !Forbidden("copy t from program 'id'") {
		t.Fatal("危险语句应被拒绝")
	}
	if Forbidden("select 1") {
		t.Fatal("普通查询不应被拒绝")
	}
}
