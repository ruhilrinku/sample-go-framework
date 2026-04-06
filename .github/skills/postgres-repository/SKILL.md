---
name: postgres-repository
description: "Use when: creating a PostgreSQL repository adapter, implementing persistence layer, adding GORM database access with multi-tenancy. Covers reader/writer DB separation, tenant scoping, soft deletes, UUID v7, BaseModel embedding, data model creation, and converter functions."
argument-hint: "Provide the feature name and its domain fields"
---

# PostgreSQL Repository Adapter

## When to Use

- Creating a new persistence layer for a feature
- User says "add repository", "implement database layer", "add postgres adapter"
- Adding new database operations to an existing feature

## Architecture

Repository adapter files live in `internal/<feature>/adapter/postgres/`:
```
<feature>_repository.go      # GORM queries
<feature>_data_model.go      # GORM model + hooks
<feature>_converter.go       # Domain ↔ Data model mappers
```

## Procedure

### 1. Create Data Model

File: `internal/<feature>/adapter/postgres/<feature>_data_model.go`

```go
package postgres

import (
	"github.com/sample-go/item-service/config"
	"github.com/sample-go/item-service/config/postgres/common"
	"gorm.io/gorm"
)

type <Feature>DataModel struct {
	ID          string `gorm:"column:id;type:uuid;primaryKey"`
	Name        string `gorm:"column:name;type:varchar(255);not null"`
	Description string `gorm:"column:description;type:text;not null;default:''"`
	// ... business fields with GORM tags
	config.BaseModel  // Embeds: TenantID, CreatedAt, CreatedBy, ModifiedAt, ModifiedBy, IsDeleted
}

func (<Feature>DataModel) TableName() string {
	return "<features>"  // plural snake_case
}

func (m *<Feature>DataModel) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		id, err := common.GenerateUUID()
		if err != nil {
			return err
		}
		m.ID = id.String()
	}
	return nil
}
```

Rules:
- Always embed `config.BaseModel` — provides tenant_id, audit timestamps, soft delete
- `TableName()` returns the plural snake_case name matching the DB migration
- `BeforeCreate` generates UUID v7 only if ID is empty (supports pre-set IDs for testing)
- GORM tags: `column:<name>`, `type:<pg_type>`, `not null`, `default:'<value>'`

### 2. Create Converter Functions

File: `internal/<feature>/adapter/postgres/<feature>_converter.go`

```go
package postgres

import (
	"github.com/sample-go/item-service/config"
	"github.com/sample-go/item-service/internal/<feature>/core/domain"
)

func toDomainModel(data <Feature>DataModel) domain.<Feature>DomainModel {
	return domain.<Feature>DomainModel{
		ID:          data.ID,
		Name:        data.Name,
		Description: data.Description,
	}
}

func toDataModel(dm domain.<Feature>DomainModel, baseModel config.BaseModel) <Feature>DataModel {
	return <Feature>DataModel{
		ID:          dm.ID,
		Name:        dm.Name,
		Description: dm.Description,
		BaseModel:   baseModel,
	}
}
```

Rules:
- `toDomainModel`: Data model → Domain model (strips infrastructure fields)
- `toDataModel`: Domain model → Data model (injects BaseModel from context)
- These are package-private functions (lowercase) — only used within the adapter
- `toDataModel` accepts `config.BaseModel` param — caller constructs it from context

### 3. Create Repository

File: `internal/<feature>/adapter/postgres/<feature>_repository.go`

```go
package postgres

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sample-go/item-service/config"
	"github.com/sample-go/item-service/config/postgres/tenant"
	"github.com/sample-go/item-service/internal/<feature>/core/domain"
	"github.com/sample-go/item-service/internal/session"
)

type <Feature>Repository struct {
	readerDB *gorm.DB
	writerDB *gorm.DB
	logger   *slog.Logger
}

func New<Feature>Repository(readerDB, writerDB *gorm.DB, logger *slog.Logger) *<Feature>Repository {
	return &<Feature>Repository{
		readerDB: readerDB,
		writerDB: writerDB,
		logger:   logger.With("component", "postgres-repo"),
	}
}
```

### Read Operations (use readerDB)

```go
func (r *<Feature>Repository) List<Feature>s(ctx context.Context, page, pageSize int) ([]domain.<Feature>DomainModel, int, error) {
	offset := (page - 1) * pageSize

	// Count total
	var total int64
	if err := r.readerDB.WithContext(ctx).
		Model(&<Feature>DataModel{}).
		Scopes(tenant.Scope(ctx)).
		Where("is_deleted = ?", false).
		Count(&total).Error; err != nil {
		return nil, 0, domain.NewInternalError("counting <features>", err)
	}

	// Fetch page
	var dataItems []<Feature>DataModel
	if err := r.readerDB.WithContext(ctx).
		Scopes(tenant.Scope(ctx)).
		Where("is_deleted = ?", false).
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&dataItems).Error; err != nil {
		return nil, 0, domain.NewInternalError("querying <features>", err)
	}

	// Convert to domain models
	items := make([]domain.<Feature>DomainModel, 0, len(dataItems))
	for _, data := range dataItems {
		items = append(items, toDomainModel(data))
	}
	return items, int(total), nil
}
```

### Write Operations (use writerDB)

```go
func (r *<Feature>Repository) Create<Feature>(ctx context.Context, item domain.<Feature>DomainModel) (domain.<Feature>DomainModel, error) {
	baseModel, err := r.setCreatedContext(ctx)
	if err != nil {
		return domain.<Feature>DomainModel{}, domain.NewInternalError("missing tenant context", err)
	}
	data := toDataModel(item, baseModel)

	if err := r.writerDB.WithContext(ctx).Create(&data).Error; err != nil {
		return domain.<Feature>DomainModel{}, domain.NewInternalError("inserting <feature>", err)
	}
	return toDomainModel(data), nil
}
```

### setCreatedContext Helper

Every write operation must build a `config.BaseModel` with tenant and audit info:
```go
func (*<Feature>Repository) setCreatedContext(ctx context.Context) (config.BaseModel, error) {
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
```

## Critical Rules

| Rule | Details |
|---|---|
| **Tenant isolation** | Every query MUST use `Scopes(tenant.Scope(ctx))` |
| **Soft deletes** | Every read MUST filter `Where("is_deleted = ?", false)` |
| **Reader for reads** | All SELECT queries use `r.readerDB` |
| **Writer for writes** | All INSERT/UPDATE/DELETE use `r.writerDB` |
| **Error wrapping** | All DB errors wrapped with `domain.NewInternalError(msg, err)` |
| **Audit fields** | Writes populate TenantID + CreatedBy from context via `setCreatedContext` |
| **UUID v7** | Generated in GORM `BeforeCreate` hook, never in domain or service layer |
| **Structured logging** | Use `DebugContext` for queries, `ErrorContext` for failures |
