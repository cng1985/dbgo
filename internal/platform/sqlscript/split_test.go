package sqlscript

import (
	"strings"
	"testing"
)

func TestSplitKeepsSemicolonInsideLiteral(t *testing.T) {
	got := Split("insert into city (name) values ('a;b');\nselect 1;")
	if len(got) != 2 {
		t.Fatalf("语句数量 = %d，内容 %#v", len(got), got)
	}
	if got[0] != "insert into city (name) values ('a;b')" {
		t.Fatalf("第一条被错误切开: %s", got[0])
	}
}

func TestSplitSkipsComments(t *testing.T) {
	got := Split("-- comment;\nselect 1; /* block; */ select 2")
	if len(got) != 2 {
		t.Fatalf("语句数量 = %d，内容 %#v", len(got), got)
	}
}

func TestSplitKeepsRoutineBody(t *testing.T) {
	script := "CREATE PROCEDURE city_size()\nBEGIN\n  SELECT 1;\n  SELECT 2;\nEND;\nSELECT 3"
	got := Split(script)
	if len(got) != 2 {
		t.Fatalf("过程体不应被切开: %#v", got)
	}
	if !strings.Contains(got[0], "SELECT 2") || !strings.Contains(got[0], "END") {
		t.Fatalf("过程语句不完整: %s", got[0])
	}
	if got[1] != "SELECT 3" {
		t.Fatalf("过程之后的语句 = %s", got[1])
	}
}

func TestSplitKeepsDollarQuote(t *testing.T) {
	got := Split("CREATE FUNCTION add(a int) RETURNS int AS $$ SELECT a; $$ LANGUAGE sql; SELECT 1")
	if len(got) != 2 {
		t.Fatalf("美元引号不应被切开: %#v", got)
	}
}

func TestSplitDoesNotTreatColumnBeginAsBlock(t *testing.T) {
	got := Split("select begin from city; delete from city")
	if len(got) != 2 {
		t.Fatalf("列名 begin 不应吞掉下一条语句: %#v", got)
	}
}

func TestSplitCaseEnd(t *testing.T) {
	got := Split("select case when 1 then 1 end; select 2")
	if len(got) != 2 {
		t.Fatalf("CASE 结束后应继续切分: %#v", got)
	}
}
