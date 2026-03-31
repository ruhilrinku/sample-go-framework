package postgres

import (
	"github.com/google/uuid"
	"github.com/sample-go/item-service/internal/config"
	"gorm.io/gorm"
)

// ItemDataModel represents the data model for the Item entity in the database.
type ItemDataModel struct {
	ID          string `gorm:"column:id;type:uuid;primaryKey"`
	Name        string `gorm:"column:name;type:varchar(255);not null"`
	Description string `gorm:"column:description;type:text;not null;default:''"`
	config.BaseModel
}

// TableName overrides the default GORM table name.
func (ItemDataModel) TableName() string {
	return "items"
}

// BeforeCreate generates a UUID v7 for the ID before inserting.
func (m *ItemDataModel) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		m.ID = id.String()
	}
	return nil
}
