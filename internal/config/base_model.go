package config

import "time"

// BaseModel contains common metadata fields for all models
type BaseModel struct {
	CreatedAt time.Time `gorm:"column:created_at"`
	CreatedBy string    `gorm:"column:created_by;type:varchar(255);not null"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
	UpdatedBy string    `gorm:"column:updated_by;type:varchar(255);not null"`
	IsDeleted bool      `gorm:"column:is_deleted;not null;default:false"`
}
