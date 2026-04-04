package config

import (
	"time"

	"github.com/google/uuid"
)

// BaseModel contains common metadata fields for all models
type BaseModel struct {
	TenantID   uuid.UUID  `gorm:"column:tenant_id;type:uuid;not null"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
	CreatedBy  uuid.UUID  `gorm:"column:created_by;type:uuid"`
	ModifiedAt *time.Time `gorm:"column:modified_at"`
	ModifiedBy *uuid.UUID `gorm:"column:modified_by;type:uuid"`
	IsDeleted  bool       `gorm:"column:is_deleted;not null;default:false"`
}
