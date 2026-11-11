package snippet

import "time"

type Snippet struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	Name        string    `gorm:"size:80;not null"`
	Description string    `gorm:"size:500;not null"`
	Body        string    `gorm:"type:text;not null"`
	Driver      string    `gorm:"size:32;not null"`
	Version     int64     `gorm:"not null"`
	CreatedBy   int64     `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (Snippet) TableName() string { return "sql_snippet" }
