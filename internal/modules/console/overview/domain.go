package overview

import "time"

type OverviewSearch struct{}

type OverviewQuery struct {
	ID             int64     `json:"id,string"`
	ConnectionName string    `json:"connection_name"`
	Status         string    `json:"status"`
	RowCount       int64     `json:"row_count"`
	DurationMS     int64     `json:"duration_ms"`
	CreatedAt      time.Time `json:"created_at"`
}

type OverviewResponse struct {
	ConnectionCount       int64           `json:"connection_count,string"`
	QueryCount            int64           `json:"query_count,string"`
	FailedQueryCount      int64           `json:"failed_query_count,string"`
	AppliedMigrationCount int64           `json:"applied_migration_count,string"`
	BackupCount           int64           `json:"backup_count,string"`
	RecentQueries         []OverviewQuery `json:"recent_queries"`
}
