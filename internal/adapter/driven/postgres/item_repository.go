package postgres

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/sample-go/item-service/internal/core/domain"
)

// ItemRepository is the PostgreSQL adapter implementing port.ItemRepository.
type ItemRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewItemRepository creates a new GORM-backed item repository.
func NewItemRepository(db *gorm.DB, logger *slog.Logger) *ItemRepository {
	return &ItemRepository{db: db, logger: logger.With("component", "postgres-repo")}
}

// ListItems retrieves a paginated list of items from PostgreSQL.
func (r *ItemRepository) ListItems(ctx context.Context, page, pageSize int) ([]domain.ItemDomainModel, int, error) {
	offset := (page - 1) * pageSize

	r.logger.DebugContext(ctx, "counting items")
	var total int64
	if err := r.db.WithContext(ctx).Model(&ItemDataModel{}).Where("is_deleted = ?", false).Count(&total).Error; err != nil {
		r.logger.ErrorContext(ctx, "failed to count items", "error", err)
		return nil, 0, domain.NewInternalError("counting items", err)
	}

	r.logger.DebugContext(ctx, "querying items", "limit", pageSize, "offset", offset)
	var dataItems []ItemDataModel
	if err := r.db.WithContext(ctx).
		Where("is_deleted = ?", false).
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&dataItems).Error; err != nil {
		r.logger.ErrorContext(ctx, "failed to query items", "error", err)
		return nil, 0, domain.NewInternalError("querying items", err)
	}

	items := make([]domain.ItemDomainModel, 0, len(dataItems))
	for _, data := range dataItems {
		items = append(items, toDomainModel(data))
	}

	r.logger.DebugContext(ctx, "fetched items", "count", len(items), "total", total)
	return items, int(total), nil
}

// CreateItem inserts a new item into PostgreSQL and returns the created item.
func (r *ItemRepository) CreateItem(ctx context.Context, item domain.ItemDomainModel) (domain.ItemDomainModel, error) {
	data := toDataModel(item)
	r.setCreatedContext(data)

	r.logger.DebugContext(ctx, "inserting item Details", "item", data)
	if err := r.db.WithContext(ctx).Create(&data).Error; err != nil {
		r.logger.ErrorContext(ctx, "failed to insert item", "name", data.Name, "error", err)
		return domain.ItemDomainModel{}, domain.NewInternalError("inserting item", err)
	}

	r.logger.DebugContext(ctx, "inserted item", "id", data.ID)
	return toDomainModel(data), nil
}

func (*ItemRepository) setCreatedContext(data ItemDataModel) {
	data.CreatedBy = "System"
	data.CreatedAt = time.Now()
}
