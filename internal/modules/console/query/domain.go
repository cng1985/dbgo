package query

import "time"

type QueryExecuteRequest struct {
	ConnectionID   int64  `json:"connection_id,string"`
	SQL            string `json:"sql"`
	ReadOnly       *bool  `json:"read_only"`
	MaxRows        int    `json:"max_rows"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type QueryOneRequest struct {
	ID      int64 `json:"id,string"`
	ActorID int64 `json:"-"`
}

type QuerySearch struct {
	ConnectionID int64  `json:"connection_id,string"`
	Status       string `json:"status"`
	Page         int    `json:"page"`
	PageSize     int    `json:"page_size"`
	ActorID      int64  `json:"-"`
}

type QueryColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type QueryStatement struct {
	Command      string        `json:"command"`
	Columns      []QueryColumn `json:"columns"`
	Rows         [][]*string   `json:"rows"`
	RowsAffected int64         `json:"rows_affected,string"`
	Truncated    bool          `json:"truncated"`
	Messages     []string      `json:"messages,omitempty"`
	SQL          string        `json:"sql,omitempty"`
}

type QuerySimple struct {
	ID             int64     `json:"id,string"`
	ConnectionID   int64     `json:"connection_id,string"`
	ConnectionName string    `json:"connection_name"`
	Status         string    `json:"status"`
	RowCount       int64     `json:"row_count"`
	DurationMS     int64     `json:"duration_ms"`
	ReadOnly       bool      `json:"read_only"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type QueryResponse struct {
	ID             int64            `json:"id,string"`
	ConnectionID   int64            `json:"connection_id,string"`
	ConnectionName string           `json:"connection_name"`
	SQL            string           `json:"sql"`
	Status         string           `json:"status"`
	ReadOnly       bool             `json:"read_only"`
	DurationMS     int64            `json:"duration_ms"`
	RowCount       int64            `json:"row_count"`
	ErrorMessage   string           `json:"error_message,omitempty"`
	Statements     []QueryStatement `json:"statements"`
	CreatedAt      time.Time        `json:"created_at"`
}
