package auth

import "time"

type User struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	Username     string    `gorm:"size:64;not null"`
	PasswordHash string    `gorm:"size:100;not null"`
	DisplayName  string    `gorm:"size:80;not null"`
	RoleCode     string    `gorm:"size:32;not null"`
	Disabled     bool      `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (User) TableName() string { return "app_user" }

type Session struct {
	ID        int64      `gorm:"primaryKey;autoIncrement"`
	UserID    int64      `gorm:"not null;index"`
	TokenHash string     `gorm:"size:64;not null"`
	ExpiresAt time.Time  `gorm:"not null"`
	RevokedAt *time.Time `gorm:""`
	CreatedAt time.Time  `gorm:"not null"`
}

func (Session) TableName() string { return "app_session" }
