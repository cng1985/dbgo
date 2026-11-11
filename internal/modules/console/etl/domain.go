package etl

import "time"

const (
	maxTasks       = 200
	maxSQLLength   = 20000
	maxMappings    = 64
	maxExtractRows = 100000
	previewRows    = 20
	insertBatch    = 100
	codeETL        = 10007
)

type ColumnMap struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type TaskRequest struct {
	ID                 int64       `json:"id,string"`
	Name               string      `json:"name"`
	SourceConnectionID int64       `json:"source_connection_id,string"`
	SourceSQL          string      `json:"source_sql"`
	TargetConnectionID int64       `json:"target_connection_id,string"`
	TargetSchema       string      `json:"target_schema"`
	TargetTable        string      `json:"target_table"`
	WriteMode          string      `json:"write_mode"`
	Mappings           []ColumnMap `json:"mappings"`
	Version            int64       `json:"version"`
}

type TaskSearch struct {
	Keyword   string `json:"keyword"`
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
	Sort      string `json:"sort"`
	Direction string `json:"direction"`
}

type TaskActionRequest struct {
	ID int64 `json:"id,string"`
}

type PreviewRequest struct {
	SourceConnectionID int64  `json:"source_connection_id,string"`
	SourceSQL          string `json:"source_sql"`
}

type RunSearch struct {
	TaskID   int64 `json:"task_id,string"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

type TaskSimple struct {
	ID                   int64      `json:"id,string"`
	Name                 string     `json:"name"`
	SourceConnectionID   int64      `json:"source_connection_id,string"`
	SourceConnectionName string     `json:"source_connection_name"`
	TargetConnectionID   int64      `json:"target_connection_id,string"`
	TargetConnectionName string     `json:"target_connection_name"`
	TargetSchema         string     `json:"target_schema"`
	TargetTable          string     `json:"target_table"`
	WriteMode            string     `json:"write_mode"`
	Version              int64      `json:"version"`
	UpdatedAt            time.Time  `json:"updated_at"`
	LastStatus           string     `json:"last_status"`
	LastRowsRead         int64      `json:"last_rows_read"`
	LastRowsWritten      int64      `json:"last_rows_written"`
	LastError            string     `json:"last_error"`
	LastFinishedAt       *time.Time `json:"last_finished_at,omitempty"`
}

type TaskResponse struct {
	ID                   int64       `json:"id,string"`
	Name                 string      `json:"name"`
	SourceConnectionID   int64       `json:"source_connection_id,string"`
	SourceConnectionName string      `json:"source_connection_name"`
	SourceSQL            string      `json:"source_sql"`
	TargetConnectionID   int64       `json:"target_connection_id,string"`
	TargetConnectionName string      `json:"target_connection_name"`
	TargetSchema         string      `json:"target_schema"`
	TargetTable          string      `json:"target_table"`
	WriteMode            string      `json:"write_mode"`
	Mappings             []ColumnMap `json:"mappings"`
	Version              int64       `json:"version"`
	CreatedAt            time.Time   `json:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at"`
}

type RunResponse struct {
	ID           int64      `json:"id,string"`
	TaskID       int64      `json:"task_id,string"`
	Status       string     `json:"status"`
	RowsRead     int64      `json:"rows_read"`
	RowsWritten  int64      `json:"rows_written"`
	ErrorMessage string     `json:"error_message"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}

type PreviewResponse struct {
	Columns   []string    `json:"columns"`
	Rows      [][]*string `json:"rows"`
	Truncated bool        `json:"truncated"`
}

type ActionResponse struct {
	Status string `json:"status"`
}
