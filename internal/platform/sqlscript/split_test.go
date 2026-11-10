package sqlscript

import "testing"

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
