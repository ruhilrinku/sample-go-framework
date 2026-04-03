package postgres

import (
	"github.com/google/uuid"
	"github.com/sample-go/item-service/internal/config/postgres/common"
	"gorm.io/gorm"
)

type PlatformFDSIdentifierMappingDataModel struct {
	ID               string    `gorm:"column:id;type:uuid;primaryKey"`
	FDSTenantID      string    `gorm:"column:fds_tenant_id;type:varchar(255);not null"`
	FDSUserID        string    `gorm:"column:fds_user_id;type:varchar(255);not null"`
	PlatformTenantID uuid.UUID `gorm:"column:platform_tenant_id;type:uuid;not null"`
	PlatformUserID   uuid.UUID `gorm:"column:platform_user_id;type:uuid;not null"`
}

func (PlatformFDSIdentifierMappingDataModel) TableName() string {
	return "platform_fds_identifier_mapping"
}

// BeforeCreate generates a UUID v7 for the ID before inserting.
func (m *PlatformFDSIdentifierMappingDataModel) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		id, err := common.GenerateUUID()
		if err != nil {
			return err
		}
		m.ID = id.String()
	}
	return nil
}
