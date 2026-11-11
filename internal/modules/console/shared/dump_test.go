package shared

import "testing"

func TestLiteralUsesHexForParameterizedBinary(t *testing.T) {
	value := []byte{0x00, 0xff}
	cases := []struct {
		driver string
		typ    string
		want   string
	}{
		{driver: "mysql", typ: "binary(16)", want: "X'00ff'"},
		{driver: "mysql", typ: "varbinary(255)", want: "X'00ff'"},
		{driver: "sqlserver", typ: "varbinary(max)", want: "0x00ff"},
		{driver: "sqlserver", typ: "BINARY(8)", want: "0x00ff"},
		{driver: "postgres", typ: "bytea", want: "'\\x00ff'"},
	}
	for _, tc := range cases {
		got, err := literal(tc.driver, value, tc.typ)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.driver, tc.typ, err)
		}
		if got != tc.want {
			t.Fatalf("%s %s 应为 %s，实际 %s", tc.driver, tc.typ, tc.want, got)
		}
	}
	text, err := literal("mysql", value, "varchar(20)")
	if err != nil {
		t.Fatal(err)
	}
	if text == "X'00ff'" {
		t.Fatal("字符类型不应按二进制十六进制写出")
	}
}
