package postgres

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sample-go/item-service/config"
	"github.com/sample-go/item-service/config/postgres/tenant"
	"github.com/sample-go/item-service/internal/items/core/domain"
	"github.com/sample-go/item-service/config/session"
)

// ItemRepository is the PostgreSQL adapter implementing port.ItemRepository.
type ItemRepository struct {
	readerDB *gorm.DB
	writerDB *gorm.DB
	logger   *slog.Logger
}

// NewItemRepository creates a new GORM-backed item repository with separate reader and writer databases.
func NewItemRepository(readerDB, writerDB *gorm.DB, logger *slog.Logger) *ItemRepository {
	return &ItemRepository{readerDB: readerDB, writerDB: writerDB, logger: logger.With("component", "postgres-repo")}
}

// ListItems retrieves a paginated list of items from PostgreSQL within the tenant boundary.
func (r *ItemRepository) ListItems(ctx context.Context, page, pageSize int) ([]domain.ItemDomainModel, int, error) {
	offset := (page - 1) * pageSize

	r.logger.DebugContext(ctx, "counting items")
	var total int64
	if err := r.readerDB.WithContext(ctx).
		Model(&ItemDataModel{}).
		Scopes(tenant.Scope(ctx)).
		Where("is_deleted = ?", false).
		Count(&total).Error; err != nil {
		r.logger.ErrorContext(ctx, "failed to count items", "error", err)
		return nil, 0, domain.NewInternalError("counting items", err)
	}

	r.logger.DebugContext(ctx, "querying items", "limit", pageSize, "offset", offset)
	var dataItems []ItemDataModel
	if err := r.readerDB.WithContext(ctx).
		Scopes(tenant.Scope(ctx)).
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
	baseModel, err := r.setCreatedContext(ctx)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to set created context", "error", err)
		return domain.ItemDomainModel{}, domain.NewInternalError("missing tenant context", err)
	}
	data := toDataModel(item, baseModel)

	r.logger.DebugContext(ctx, "inserting item Details", "item", data)
	if err := r.writerDB.WithContext(ctx).Create(&data).Error; err != nil {
		r.logger.ErrorContext(ctx, "failed to insert item", "name", data.Name, "error", err)
		return domain.ItemDomainModel{}, domain.NewInternalError("inserting item", err)
	}

	r.logger.DebugContext(ctx, "inserted item", "id", data.ID)
	return toDomainModel(data), nil
}

func (*ItemRepository) setCreatedContext(ctx context.Context) (config.BaseModel, error) {
	tenantID, err := tenant.SetTenantID(ctx)
	if err != nil {
		return config.BaseModel{}, err
	}
	createdBy := uuid.Nil
	if sess := session.FromContext(ctx); sess != nil && sess.UserID != uuid.Nil {
		createdBy = sess.UserID
	}
	return config.BaseModel{
		TenantID:  tenantID,
		CreatedAt: time.Now(),
		CreatedBy: createdBy,
	}, nil
}
