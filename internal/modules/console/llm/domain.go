package llm

import "time"

type ProfileRequest struct {
	ID          int64   `json:"id,string"`
	Name        string  `json:"name"`
	BaseURL     string  `json:"base_url"`
	APIKey      string  `json:"api_key"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	IsDefault   bool    `json:"is_default"`
	Remark      string  `json:"remark"`
	Version     int64   `json:"version"`
}

type ProfileDeleteRequest struct {
	ID int64 `json:"id,string"`
}

type ProfileSearch struct {
	Keyword   string `json:"keyword"`
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
	Sort      string `json:"sort"`
	Direction string `json:"direction"`
}

type ProfileSimple struct {
	ID          int64     `json:"id,string"`
	Name        string    `json:"name"`
	BaseURL     string    `json:"base_url"`
	Model       string    `json:"model"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	IsDefault   bool      `json:"is_default"`
	APIKeySet   bool      `json:"api_key_set"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProfileResponse struct {
	ID          int64     `json:"id,string"`
	Name        string    `json:"name"`
	BaseURL     string    `json:"base_url"`
	Model       string    `json:"model"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	IsDefault   bool      `json:"is_default"`
	Remark      string    `json:"remark"`
	APIKeySet   bool      `json:"api_key_set"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TestResponse struct {
	OK        bool   `json:"ok"`
	Detail    string `json:"detail"`
	LatencyMS int64  `json:"latency_ms"`
}

type OptimizeRequest struct {
	ConnectionID int64  `json:"connection_id,string"`
	ProfileID    int64  `json:"profile_id,string"`
	SQL          string `json:"sql"`
	Schema       string `json:"schema"`
}

type OptimizeResponse struct {
	ProfileName string   `json:"profile_name"`
	Model       string   `json:"model"`
	Summary     string   `json:"summary"`
	SQL         string   `json:"sql"`
	Notes       []string `json:"notes"`
}

type DesignTableRequest struct {
	ConnectionID int64  `json:"connection_id,string"`
	ProfileID    int64  `json:"profile_id,string"`
	Schema       string `json:"schema"`
	Prompt       string `json:"prompt"`
}

type DesignColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Primary  bool   `json:"primary"`
	Identity bool   `json:"identity"`
	Default  string `json:"default"`
	Comment  string `json:"comment"`
}

type DesignTableResponse struct {
	ProfileName string         `json:"profile_name"`
	Model       string         `json:"model"`
	Summary     string         `json:"summary"`
	Name        string         `json:"name"`
	Comment     string         `json:"comment"`
	Columns     []DesignColumn `json:"columns"`
	Notes       []string       `json:"notes"`
}
