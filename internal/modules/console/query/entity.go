package query

import "time"

type QueryHistory struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	ConnectionID   int64     `gorm:"not null;index"`
	ConnectionName string    `gorm:"size:80;not null"`
	ActorID        int64     `gorm:"column:actor_id;not null"`
	SQLText        string    `gorm:"column:sql_text;type:text;not null"`
	SchemaName     string    `gorm:"column:schema_name;size:128;not null"`
	Status         string    `gorm:"size:16;not null"`
	RowCount       int64     `gorm:"not null"`
	DurationMS     int64     `gorm:"column:duration_ms;not null"`
	ErrorMessage   string    `gorm:"size:2000;not null"`
	ReadOnly       bool      `gorm:"not null"`
	CreatedAt      time.Time `gorm:"not null"`
}

func (QueryHistory) TableName() string { return "query_history" }
