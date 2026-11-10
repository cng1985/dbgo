package migration

import "time"

type MigrationRequest struct {
	ID           int64  `json:"id,string"`
	ConnectionID int64  `json:"connection_id,string"`
	VersionNo    int64  `json:"version_no"`
	Name         string `json:"name"`
	UpSQL        string `json:"up_sql"`
	DownSQL      string `json:"down_sql"`
}

type MigrationActionRequest struct {
	ID           int64  `json:"id,string"`
	ConnectionID int64  `json:"connection_id,string"`
	Action       string `json:"action"`
}

type MigrationSearch struct {
	ConnectionID int64  `json:"connection_id,string"`
	Status       string `json:"status"`
	Page         int    `json:"page"`
	PageSize     int    `json:"page_size"`
}

type MigrationSimple struct {
	ID             int64      `json:"id,string"`
	ConnectionID   int64      `json:"connection_id,string"`
	ConnectionName string     `json:"connection_name"`
	VersionNo      int64      `json:"version_no"`
	Name           string     `json:"name"`
	Status         string     `json:"status"`
	Checksum       string     `json:"checksum"`
	AppliedAt      *time.Time `json:"applied_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type MigrationResponse struct {
	ID             int64      `json:"id,string"`
	ConnectionID   int64      `json:"connection_id,string"`
	ConnectionName string     `json:"connection_name"`
	VersionNo      int64      `json:"version_no"`
	Name           string     `json:"name"`
	UpSQL          string     `json:"up_sql"`
	DownSQL        string     `json:"down_sql"`
	Checksum       string     `json:"checksum"`
	Status         string     `json:"status"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	AppliedAt      *time.Time `json:"applied_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
