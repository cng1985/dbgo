package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Endpoint struct {
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   int
}

func NormalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Hostname() == "" {
		return "", apperrors.Invalid("模型服务地址不正确")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", apperrors.Invalid("模型服务只支持 http 或 https")
	}
	if parsed.User != nil {
		return "", apperrors.Invalid("模型服务地址不能包含账号")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "169.254.169.254" || host == "metadata.google.internal" {
		return "", apperrors.Invalid("模型服务地址不被允许")
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLinkLocalUnicast() {
		return "", apperrors.Invalid("模型服务地址不被允许")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func Complete(ctx context.Context, endpoint Endpoint, messages []Message) (string, error) {
	body := map[string]any{
		"model":       endpoint.Model,
		"temperature": endpoint.Temperature,
		"messages":    messages,
	}
	if endpoint.MaxTokens > 0 {
		body["max_tokens"] = endpoint.MaxTokens
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	raw, err := doJSON(ctx, http.MethodPost, endpoint.BaseURL+"/chat/completions", endpoint.APIKey, payload)
	if err != nil {
		return "", err
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("模型响应不是 JSON")
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", fmt.Errorf("%s", clip(parsed.Error.Message))
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("模型没有返回内容")
	}
	return parsed.Choices[0].Message.Content, nil
}

func Probe(ctx context.Context, endpoint Endpoint) (string, error) {
	raw, status, err := do(ctx, http.MethodGet, endpoint.BaseURL+"/models", endpoint.APIKey, nil)
	if err == nil && status < 300 {
		var parsed struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &parsed) == nil && len(parsed.Data) > 0 {
			return fmt.Sprintf("已连通，看到 %d 个模型", len(parsed.Data)), nil
		}
		return "已连通模型服务", nil
	}
	text, err := Complete(ctx, Endpoint{
		BaseURL:     endpoint.BaseURL,
		APIKey:      endpoint.APIKey,
		Model:       endpoint.Model,
		Temperature: 0,
		MaxTokens:   8,
	}, []Message{{Role: "user", Content: "回复 OK"}})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("模型没有返回内容")
	}
	return "对话接口可用", nil
}

func doJSON(ctx context.Context, method, address, apiKey string, payload []byte) ([]byte, error) {
	raw, status, err := do(ctx, method, address, apiKey, payload)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("模型服务返回 HTTP %d：%s", status, clip(string(raw)))
	}
	return raw, nil
}

func do(ctx context.Context, method, address, apiKey string, payload []byte) ([]byte, int, error) {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, address, reader)
	if err != nil {
		return nil, 0, err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
	request.Header.Set("Accept", "application/json")
	client := &http.Client{
		Timeout: 50 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("重定向过多")
			}
			if len(via) > 0 && !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
				return fmt.Errorf("拒绝跨主机重定向")
			}
			return nil
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("无法连接模型服务")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, response.StatusCode, err
	}
	return body, response.StatusCode, nil
}

func clip(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 240 {
		return value[:240]
	}
	return value
}
