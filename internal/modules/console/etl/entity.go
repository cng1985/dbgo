package etl

import "time"

type Task struct {
	ID                   int64     `gorm:"primaryKey;autoIncrement"`
	Name                 string    `gorm:"size:80;not null"`
	SourceConnectionID   int64     `gorm:"not null"`
	SourceConnectionName string    `gorm:"size:80;not null"`
	SourceSQL            string    `gorm:"column:source_sql;type:text;not null"`
	TargetConnectionID   int64     `gorm:"not null"`
	TargetConnectionName string    `gorm:"size:80;not null"`
	TargetSchema         string    `gorm:"size:128;not null"`
	TargetTable          string    `gorm:"size:128;not null"`
	WriteMode            string    `gorm:"size:16;not null"`
	Mappings             string    `gorm:"type:text;not null"`
	Version              int64     `gorm:"not null"`
	CreatedBy            int64     `gorm:"not null"`
	CreatedAt            time.Time `gorm:"not null"`
	UpdatedAt            time.Time `gorm:"not null"`
}

func (Task) TableName() string { return "etl_task" }

type Run struct {
	ID           int64      `gorm:"primaryKey;autoIncrement"`
	TaskID       int64      `gorm:"not null;index"`
	Status       string     `gorm:"size:16;not null"`
	RowsRead     int64      `gorm:"not null"`
	RowsWritten  int64      `gorm:"not null"`
	ErrorMessage string     `gorm:"size:2000;not null"`
	StartedAt    time.Time  `gorm:"not null"`
	FinishedAt   *time.Time `gorm:""`
	CreatedBy    int64      `gorm:"not null"`
}

func (Run) TableName() string { return "etl_run" }
