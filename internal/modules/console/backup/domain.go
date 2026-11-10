package backup

import "time"

type BackupRequest struct {
	ID           int64  `json:"id,string"`
	ConnectionID int64  `json:"connection_id,string"`
	Name         string `json:"name"`
	Format       string `json:"format"`
}

type BackupSearch struct {
	ConnectionID int64  `json:"connection_id,string"`
	Status       string `json:"status"`
	Page         int    `json:"page"`
	PageSize     int    `json:"page_size"`
}

type BackupSimple struct {
	ID             int64     `json:"id,string"`
	ConnectionID   int64     `json:"connection_id,string"`
	ConnectionName string    `json:"connection_name"`
	Driver         string    `json:"driver"`
	Name           string    `json:"name"`
	Format         string    `json:"format"`
	SizeBytes      int64     `json:"size_bytes,string"`
	Checksum       string    `json:"checksum"`
	Status         string    `json:"status"`
	Truncated      bool      `json:"truncated"`
	CreatedAt      time.Time `json:"created_at"`
}

type BackupResponse struct {
	ID             int64     `json:"id,string"`
	ConnectionID   int64     `json:"connection_id,string"`
	ConnectionName string    `json:"connection_name"`
	Driver         string    `json:"driver"`
	Name           string    `json:"name"`
	Format         string    `json:"format"`
	SizeBytes      int64     `json:"size_bytes,string"`
	Checksum       string    `json:"checksum"`
	Status         string    `json:"status"`
	Truncated      bool      `json:"truncated"`
	StatementCount int64     `json:"statement_count,string"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	Detail         string    `json:"detail,omitempty"`
	FileName       string    `json:"file_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	LocalPath      string    `json:"-"`
}
