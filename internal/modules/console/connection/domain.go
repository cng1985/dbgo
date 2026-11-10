package connection

import "time"

type ConnectionRequest struct {
	ID           int64  `json:"id,string"`
	Name         string `json:"name"`
	Driver       string `json:"driver"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	DatabaseName string `json:"database_name"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	SQLitePath   string `json:"sqlite_path"`
	SSLMode      string `json:"ssl_mode"`
	Remark       string `json:"remark"`
	Version      int64  `json:"version"`
}

type ConnectionDeleteRequest struct {
	ID int64 `json:"id,string"`
}

type ConnectionSearch struct {
	Keyword   string `json:"keyword"`
	Driver    string `json:"driver"`
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
	Sort      string `json:"sort"`
	Direction string `json:"direction"`
}

type ConnectionSimple struct {
	ID           int64     `json:"id,string"`
	Name         string    `json:"name"`
	Driver       string    `json:"driver"`
	Host         string    `json:"host"`
	Port         int       `json:"port"`
	DatabaseName string    `json:"database_name"`
	Username     string    `json:"username"`
	SQLitePath   string    `json:"sqlite_path"`
	SSLMode      string    `json:"ssl_mode"`
	PasswordSet  bool      `json:"password_set"`
	Version      int64     `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ConnectionResponse struct {
	ID            int64     `json:"id,string"`
	Name          string    `json:"name"`
	Driver        string    `json:"driver"`
	Host          string    `json:"host"`
	Port          int       `json:"port"`
	DatabaseName  string    `json:"database_name"`
	Username      string    `json:"username"`
	SQLitePath    string    `json:"sqlite_path"`
	SSLMode       string    `json:"ssl_mode"`
	Remark        string    `json:"remark"`
	PasswordSet   bool      `json:"password_set"`
	Version       int64     `json:"version"`
	Reachable     *bool     `json:"reachable,omitempty"`
	ServerVersion string    `json:"server_version,omitempty"`
	LatencyMS     int64     `json:"latency_ms"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
