package monitor

import "time"

type SnapshotSearch struct {
	ConnectionID int64  `json:"connection_id,string"`
	Schema       string `json:"schema"`
}

type Metric struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
	Unit  string `json:"unit,omitempty"`
	Hint  string `json:"hint,omitempty"`
}

type SessionInfo struct {
	ID       string `json:"id"`
	User     string `json:"user"`
	Database string `json:"database"`
	State    string `json:"state"`
	Duration string `json:"duration"`
	SQL      string `json:"sql"`
}

type SizeInfo struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Rows   int64  `json:"rows"`
	Bytes  int64  `json:"bytes"`
}

type SnapshotResponse struct {
	Driver        string        `json:"driver"`
	ServerVersion string        `json:"server_version"`
	Schema        string        `json:"schema"`
	CollectedAt   time.Time     `json:"collected_at"`
	Metrics       []Metric      `json:"metrics"`
	Sessions      []SessionInfo `json:"sessions"`
	Sizes         []SizeInfo    `json:"sizes"`
	Notes         []string      `json:"notes"`
}
