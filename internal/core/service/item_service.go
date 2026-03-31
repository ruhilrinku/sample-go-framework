package service

import (
	"context"
	"log/slog"

	"github.com/sample-go/item-service/internal/core/domain"
	"github.com/sample-go/item-service/internal/core/port"
)

// ItemService implements the primary port using injected secondary ports.
type ItemService struct {
	repo   port.ItemRepository
	logger *slog.Logger
}

// New creates a new ItemService with the given repository.
func New(repo port.ItemRepository, logger *slog.Logger) *ItemService {
	return &ItemService{repo: repo, logger: logger.With("component", "item-service")}
}

// ListItems fetches a paginated list of items.
func (itemService *ItemService) ListItems(ctx context.Context, page, pageSize int) ([]domain.ItemDomainModel, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	itemService.logger.InfoContext(ctx, "listing items", "page", page, "pageSize", pageSize)

	items, total, err := itemService.repo.ListItems(ctx, page, pageSize)
	if err != nil {
		itemService.logger.ErrorContext(ctx, "failed to list items", "error", err)
		return nil, 0, domain.NewInternalError("listing items", err)
	}

	itemService.logger.InfoContext(ctx, "listed items", "count", len(items), "total", total)
	return items, total, nil
}

// CreateItem validates input and creates a new item.
func (itemService *ItemService) CreateItem(ctx context.Context, name, description string) (domain.ItemDomainModel, error) {
	if name == "" {
		itemService.logger.WarnContext(ctx, "create item called with empty name")
		return domain.ItemDomainModel{}, domain.NewValidationError("item name is required")
	}

	itemService.logger.InfoContext(ctx, "creating item", "name", name)

	item := domain.ItemDomainModel{
		Name:        name,
		Description: description,
	}

	created, err := itemService.repo.CreateItem(ctx, item)
	if err != nil {
		itemService.logger.ErrorContext(ctx, "failed to create item", "name", name, "error", err)
		return domain.ItemDomainModel{}, domain.NewInternalError("creating item", err)
	}

	itemService.logger.InfoContext(ctx, "created item", "id", created.ID, "name", created.Name)
	return created, nil
}
