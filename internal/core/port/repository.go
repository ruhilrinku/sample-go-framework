package port

import (
	"context"

	"github.com/sample-go/item-service/internal/core/domain"
)

// ItemRepository is the driven (secondary) port for persistence.
type ItemRepository interface {
	ListItems(ctx context.Context, page, pageSize int) ([]domain.ItemDomainModel, int, error)
	CreateItem(ctx context.Context, item domain.ItemDomainModel) (domain.ItemDomainModel, error)
}
