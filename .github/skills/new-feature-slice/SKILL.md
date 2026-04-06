---
name: new-feature-slice
description: "Use when: adding a new domain feature, creating a new hexagonal slice, scaffolding a new bounded context. Creates complete feature structure with domain model, ports, service, gRPC adapter, postgres adapter, converter, data model, migration changelog, protobuf definition, and wiring in main.go."
argument-hint: "Provide the feature name (e.g., 'orders') and its domain fields"
---

# New Hexagonal Feature Slice

## When to Use

- Adding a new domain feature to the microservice
- User says "add a new entity", "create a new resource", "scaffold a feature"
- Creating a new bounded context following the hexagonal architecture

## Procedure

### 1. Gather Requirements

Ask the user for:
- **Feature name** (singular, lowercase): e.g., `order`, `category`, `tag`
- **Domain fields**: name + Go type for each business field (exclude ID, audit fields)
- **Operations needed**: which RPCs to support (List, Create, Get, Update, Delete)

### 2. Create Directory Structure

Every feature follows this exact layout under `internal/`:

```
internal/<feature>/
  core/
    domain/
      <feature>_domain_model.go    # Domain model + typed errors (reuse domain.DomainError)
      errors.go                     # Only if feature needs custom error types beyond shared ones
    port/
      repository.go                # Secondary (driven) port interface
      service.go                   # Primary (driving) port interface
    service/
      <feature>_service.go         # Business logic
      <feature>_service_test.go    # Unit tests with manual mocks
  adapter/
    grpc/
      <feature>_server.go          # gRPC handler
      <feature>_server_test.go     # Unit tests with manual mocks
    postgres/
      <feature>_repository.go      # GORM persistence
      <feature>_data_model.go      # GORM data model
      <feature>_converter.go       # Domain ↔ Data model converters
```

### 3. Domain Model

File: `internal/<feature>/core/domain/<feature>_domain_model.go`

Rules:
- Package `domain`
- Minimal business fields only — NO metadata, NO GORM tags, NO infrastructure imports
- ID is always `string` type
- Reuse `domain.DomainError` types from `internal/items/core/domain/errors.go` — do NOT duplicate

Template:
```go
package domain

// <Feature>DomainModel represents the core domain entity.
type <Feature>DomainModel struct {
	ID    string
	// ... business fields only
}
```

### 4. Port Interfaces

**Repository port** (`internal/<feature>/core/port/repository.go`):
- Package `port`
- Accepts and returns domain models ONLY
- Pagination returns `([]domain.Model, totalCount int, error)`

```go
package port

import (
	"context"
	"github.com/sample-go/item-service/internal/<feature>/core/domain"
)

type <Feature>Repository interface {
	List<Feature>s(ctx context.Context, page, pageSize int) ([]domain.<Feature>DomainModel, int, error)
	Create<Feature>(ctx context.Context, item domain.<Feature>DomainModel) (domain.<Feature>DomainModel, error)
}
```

**Service port** (`internal/<feature>/core/port/service.go`):
- Package `port`
- Accepts primitives for creation (not domain models)
- Returns domain models

```go
package port

import (
	"context"
	"github.com/sample-go/item-service/internal/<feature>/core/domain"
)

type <Feature>Service interface {
	List<Feature>s(ctx context.Context, page, pageSize int) ([]domain.<Feature>DomainModel, int, error)
	Create<Feature>(ctx context.Context, name, description string) (domain.<Feature>DomainModel, error)
}
```

### 5. Service Implementation

File: `internal/<feature>/core/service/<feature>_service.go`

Rules:
- Constructor: `func New(repo port.<Feature>Repository, logger *slog.Logger) *<Feature>Service`
- Logger tagged: `logger.With("component", "<feature>-service")`
- Input validation returns `domain.NewValidationError(msg)`
- Repository errors wrapped: `domain.NewInternalError(msg, err)`
- Default pagination: page < 1 → 1, pageSize < 1 or > 100 → 20
- Structured logging with `InfoContext`, `ErrorContext`, `WarnContext`

### 6. Data Model

File: `internal/<feature>/adapter/postgres/<feature>_data_model.go`

Rules:
- Embeds `config.BaseModel` for audit/tenant fields
- GORM tags on every field
- `TableName()` override returns plural table name
- `BeforeCreate` hook generates UUID v7 via `common.GenerateUUID()`

Template:
```go
package postgres

import (
	"github.com/sample-go/item-service/config"
	"github.com/sample-go/item-service/config/postgres/common"
	"gorm.io/gorm"
)

type <Feature>DataModel struct {
	ID   string `gorm:"column:id;type:uuid;primaryKey"`
	Name string `gorm:"column:name;type:varchar(255);not null"`
	// ... other fields with GORM tags
	config.BaseModel
}

func (<Feature>DataModel) TableName() string {
	return "<features>"  // plural
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

### 7. Converter Functions

File: `internal/<feature>/adapter/postgres/<feature>_converter.go`

```go
package postgres

import (
	"github.com/sample-go/item-service/config"
	"github.com/sample-go/item-service/internal/<feature>/core/domain"
)

func toDomainModel(data <Feature>DataModel) domain.<Feature>DomainModel {
	return domain.<Feature>DomainModel{
		ID:   data.ID,
		// ... map fields
	}
}

func toDataModel(dm domain.<Feature>DomainModel, baseModel config.BaseModel) <Feature>DataModel {
	return <Feature>DataModel{
		ID:        dm.ID,
		// ... map fields
		BaseModel: baseModel,
	}
}
```

### 8. Repository Implementation

File: `internal/<feature>/adapter/postgres/<feature>_repository.go`

Rules:
- Constructor takes `(readerDB, writerDB *gorm.DB, logger *slog.Logger)`
- Logger tagged: `logger.With("component", "postgres-repo")`
- All reads use `r.readerDB` with `tenant.Scope(ctx)` and `Where("is_deleted = ?", false)`
- All writes use `r.writerDB`
- Before writes: call `tenant.SetTenantID(ctx)` → build `config.BaseModel`
- Set `CreatedBy` from `session.FromContext(ctx).UserID`
- Wrap DB errors: `domain.NewInternalError(msg, err)`

### 9. Protobuf Definition

Add to `proto/<feature>/v1/<feature>.proto`:
- Package: `<feature>.v1`
- `go_package`: `"github.com/sample-go/item-service/gen/pb/<feature>/v1;<feature>v1"`
- HTTP annotations for grpc-gateway on every RPC
- Run `make generate` after creating

### 10. gRPC Adapter

File: `internal/<feature>/adapter/grpc/<feature>_server.go`

Rules:
- Embed `Unimplemented<Feature>ServiceServer`
- Constructor: `func New<Feature>Server(svc port.<Feature>Service, logger *slog.Logger) *<Feature>Server`
- Logger tagged: `logger.With("component", "grpc-server")`
- Map protobuf → domain at entry, domain → protobuf at exit
- Use `toGRPCError(err)` helper for domain→gRPC status translation
- Input validation at adapter level returns `status.Errorf(codes.InvalidArgument, ...)`

The `toGRPCError` helper:
```go
func toGRPCError(err error) error {
	switch domain.GetErrorType(err) {
	case domain.ErrorTypeValidation:
		return status.Error(codes.InvalidArgument, err.Error())
	case domain.ErrorTypeNotFound:
		return status.Error(codes.NotFound, err.Error())
	case domain.ErrorTypeInternal:
		return status.Error(codes.Internal, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
```

### 11. Database Migration

Add a new changelog YAML in `db-migrations/changelogs/`:
- File: `NNN-create-<features>-table.yaml` (next sequence number)
- Always include: `id` (UUID PK), `tenant_id` (UUID NOT NULL), `created_at`, `created_by`, `modified_at`, `modified_by`, `is_deleted` (BOOLEAN default false)
- Add index on `tenant_id`
- Register in `db-migrations/changelog-master.yaml`

### 12. Wire in main.go

Add to `cmd/server/main.go` following the existing pattern:
```go
// Hexagonal wiring for <feature>
<feature>Repo := <feature>Postgres.New<Feature>Repository(readerDB, writerDB, logger)
<feature>Svc := <feature>Service.New(<feature>Repo, logger)
<feature>Server := <feature>Grpc.New<Feature>Server(<feature>Svc, logger)

// Register with gRPC server
<feature>v1.Register<Feature>ServiceServer(grpcServer, <feature>Server)

// Register gateway endpoint
<feature>v1.Register<Feature>ServiceHandlerFromEndpoint(ctx, gwMux, grpcAddr, opts)
```

### 13. Verify

- Run `make generate` to regenerate protobuf
- Run `make build` to ensure compilation
- Run `make test` to run all tests
