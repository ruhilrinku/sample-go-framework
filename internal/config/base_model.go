package config

import "time"

// BaseModel contains common metadata fields for all models
type BaseModel struct {
	CreatedAt  time.Time  `gorm:"column:created_at"`
	CreatedBy  string     `gorm:"column:created_by;type:varchar(255)"`
	ModifiedAt *time.Time `gorm:"column:modified_at"`
	ModifiedBy *string    `gorm:"column:modified_by;type:varchar(255)"`
	IsDeleted  bool       `gorm:"column:is_deleted;not null;default:false"`
}
