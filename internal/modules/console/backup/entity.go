package backup

import "time"

type DataBackup struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	ConnectionID   int64     `gorm:"not null;index"`
	ConnectionName string    `gorm:"size:80;not null"`
	Driver         string    `gorm:"size:32;not null"`
	Name           string    `gorm:"size:120;not null"`
	Format         string    `gorm:"size:16;not null"`
	StorageKey     string    `gorm:"size:255;not null"`
	SizeBytes      int64     `gorm:"not null"`
	Checksum       string    `gorm:"size:64;not null"`
	Status         string    `gorm:"size:16;not null"`
	Truncated      bool      `gorm:"not null"`
	StatementCount int64     `gorm:"not null"`
	ErrorMessage   string    `gorm:"size:2000;not null"`
	CreatedBy      int64     `gorm:"not null"`
	CreatedAt      time.Time `gorm:"not null"`
}

func (DataBackup) TableName() string { return "data_backup" }
