package port

import (
	"context"

	"github.com/sample-go/item-service/internal/core/domain"
)

// ItemService is the driving (primary) port for the application use-cases.
type ItemService interface {
	ListItems(ctx context.Context, page, pageSize int) ([]domain.ItemDomainModel, int, error)
	CreateItem(ctx context.Context, name, description string) (domain.ItemDomainModel, error)
}
