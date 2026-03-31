package postgres

import "github.com/sample-go/item-service/internal/core/domain"

// toDomainModel converts an ItemDataModel to an ItemDomainModel.
func toDomainModel(data ItemDataModel) domain.ItemDomainModel {
	return domain.ItemDomainModel{
		ID:          data.ID,
		Name:        data.Name,
		Description: data.Description,
	}
}

// toDataModel converts an ItemDomainModel to an ItemDataModel.
func toDataModel(dm domain.ItemDomainModel) ItemDataModel {
	return ItemDataModel{
		ID:          dm.ID,
		Name:        dm.Name,
		Description: dm.Description,
	}
}
