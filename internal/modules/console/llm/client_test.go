package llm

import "testing"

func TestNormalizeBaseURL(t *testing.T) {
	got, err := NormalizeBaseURL("https://api.openai.com/v1/")
	if err != nil || got != "https://api.openai.com/v1" {
		t.Fatalf("应去掉末尾斜线: %q %v", got, err)
	}
	if _, err := NormalizeBaseURL("file:///tmp/model"); err == nil {
		t.Fatal("应拒绝 file 协议")
	}
	if _, err := NormalizeBaseURL("http://169.254.169.254/latest"); err == nil {
		t.Fatal("应拒绝元数据地址")
	}
	if _, err := NormalizeBaseURL("http://user:pass@127.0.0.1:11434/v1"); err == nil {
		t.Fatal("应拒绝带账号的地址")
	}
}

func TestParseAdvice(t *testing.T) {
	parsed, err := parseAdvice("```json\n{\"summary\":\"可以改写\",\"sql\":\"select id from city\",\"notes\":[\"避免 select *\"]}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.SQL != "select id from city" || parsed.Summary != "可以改写" || len(parsed.Notes) != 1 {
		t.Fatalf("解析结果不正确: %#v", parsed)
	}
	fallback, err := parseAdvice("不是 JSON")
	if err == nil || fallback.Summary == "" {
		t.Fatalf("无法解析时应保留原文摘要: %#v %v", fallback, err)
	}
}
