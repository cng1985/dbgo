package connection

import "time"

type Connection struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	Name           string    `gorm:"size:80;not null"`
	Driver         string    `gorm:"size:32;not null"`
	Host           string    `gorm:"size:255;not null"`
	Port           int       `gorm:"not null"`
	DatabaseName   string    `gorm:"size:128;not null"`
	Username       string    `gorm:"size:128;not null"`
	PasswordCipher string    `gorm:"column:password_cipher;type:text;not null"`
	SQLitePath     string    `gorm:"column:sqlite_path;size:500;not null"`
	SSLMode        string    `gorm:"column:ssl_mode;size:32;not null"`
	Remark         string    `gorm:"size:500;not null"`
	Version        int64     `gorm:"not null"`
	CreatedBy      int64     `gorm:"not null"`
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
}

func (Connection) TableName() string { return "db_connection" }
