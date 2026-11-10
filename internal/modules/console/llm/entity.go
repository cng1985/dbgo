package llm

import "time"

type Profile struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	Name         string    `gorm:"size:80;not null"`
	BaseURL      string    `gorm:"column:base_url;size:500;not null"`
	APIKeyCipher string    `gorm:"column:api_key_cipher;type:text;not null"`
	Model        string    `gorm:"size:120;not null"`
	Temperature  float64   `gorm:"not null"`
	MaxTokens    int       `gorm:"column:max_tokens;not null"`
	IsDefault    bool      `gorm:"column:is_default;not null"`
	Remark       string    `gorm:"size:500;not null"`
	Version      int64     `gorm:"not null"`
	CreatedBy    int64     `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (Profile) TableName() string { return "llm_profile" }
