package migration

import "time"

type DataMigration struct {
	ID             int64      `gorm:"primaryKey;autoIncrement"`
	ConnectionID   int64      `gorm:"not null;index"`
	ConnectionName string     `gorm:"size:80;not null"`
	VersionNo      int64      `gorm:"not null"`
	Name           string     `gorm:"size:120;not null"`
	UpSQL          string     `gorm:"column:up_sql;type:text;not null"`
	DownSQL        string     `gorm:"column:down_sql;type:text;not null"`
	Checksum       string     `gorm:"size:64;not null"`
	Status         string     `gorm:"size:16;not null"`
	ErrorMessage   string     `gorm:"size:2000;not null"`
	AppliedAt      *time.Time `gorm:""`
	CreatedBy      int64      `gorm:"not null"`
	CreatedAt      time.Time  `gorm:"not null"`
	UpdatedAt      time.Time  `gorm:"not null"`
}

func (DataMigration) TableName() string { return "data_migration" }
