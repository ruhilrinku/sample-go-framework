# Project Guidelines

## Overview

Go gRPC microservice (`github.com/sample-go/item-service`) using **hexagonal architecture** (ports & adapters). Go 1.26+, PostgreSQL, GORM, grpc-gateway, column-level multi-tenancy.

See [README.md](../README.md) for full architecture, API reference, authentication modes, and database schemas.

## Build and Test

```bash
make run          # Start server (migrations run automatically)
make build        # Build all packages
make test         # Unit + BDD tests (verbose, no cache)
make generate     # Regenerate protobuf code via buf CLI
make clean        # Remove generated protobuf code
```

Run only Cucumber/BDD tests:

```bash
go test ./internal/items/adapter/grpc/... -run TestFeatures -v
```

## Architecture

### Hexagonal Slice Structure

Each domain feature is a self-contained **hexagonal slice** under `internal/`:

```
internal/<feature>/
  core/
    domain/       # Domain models and typed errors (no infra imports)
    port/         # Interfaces: service (primary) and repository (secondary)
    service/      # Business logic implementing service port
  adapter/
    grpc/         # Driving adapter (inbound), depends on service port
    postgres/     # Driven adapter (outbound), implements repository port
```

Current slices: `items/`, `fds/`. New features must follow this structure.

### Key Boundaries

- **Ports accept and return domain models only** — never data models or protobuf types
- **Domain core has zero infrastructure imports** — no GORM, gRPC, or adapter packages
- **Adapters own all translation** — protobuf ↔ domain (gRPC adapter), domain ↔ data model (postgres adapter)
- **Error translation happens at the boundary** — adapters convert `DomainError` types to gRPC status codes via `toGRPCError()`

### Dependency Flow

```
gRPC/REST request → Session Interceptor → gRPC Adapter → Service Port → Repository Port → PostgreSQL Adapter
```

Wiring is **manual constructor injection** in `cmd/server/main.go` — no DI framework.

## Conventions

### Models

- **Domain model** (`ItemDomainModel`): Minimal business fields only — no metadata, no GORM tags
- **Data model** (`ItemDataModel`): Embeds `config.BaseModel` for audit/tenant fields, has GORM tags and `BeforeCreate` hook for UUID v7
- **Converter functions** (`toDomainModel()`, `toDataModel()`) live in the postgres adapter package

### Database

- **Reader/Writer separation**: Repositories take two `*gorm.DB` connections (`readerDB`, `writerDB`)
- **Tenant scoping**: Every query uses `tenant.Scope(ctx)` — wraps `WHERE tenant_id = ?`
- **Tenant on writes**: Call `tenant.SetTenantID(ctx)` before inserts to populate `BaseModel.TenantID`
- **Soft deletes**: `IsDeleted` field in `BaseModel`; reads filter `is_deleted = false`
- **UUID v7**: Generated via `common.GenerateUUID()` in GORM `BeforeCreate` hook, not in domain layer
- **Migrations**: Pure Go Liquibase-compatible runner (YAML changelogs in `db/`), runs on startup

### Errors

Three typed domain errors — use the factory functions:

| Factory | gRPC Code | Use For |
|---------|-----------|---------|
| `NewValidationError(msg)` | `InvalidArgument` | Bad input |
| `NewNotFoundError(msg)` | `NotFound` | Missing resource |
| `NewInternalError(msg, err)` | `Internal` | System/DB failures (wraps cause) |

### Session & Auth

- `session.FromContext(ctx)` retrieves `RequestSession` from context
- JWT Bearer tokens and explicit `tenant_id`/`user_id` headers both supported
- FDS tokens (matching `fds_issuer`) trigger platform identity resolution via `PlatformFDSIdentifierMapService`
- Trace headers (`x-b3-traceid`, `x-request-id`, etc.) extracted into `RequestSession.Traces`

### Protobuf

- Source: `proto/item/v1/item.proto` with HTTP annotations for grpc-gateway
- Generated code: `gen/pb/item/v1/` — **never edit manually**, regenerate with `make generate`
- Buf CLI config: `buf.yaml` (module) + `buf.gen.yaml` (plugins)

## Testing

### Unit Tests

- **Manual mocks** — hand-written structs implementing port interfaces (no codegen mocks)
- **Naming**: `Test{Function}_{Scenario}` — e.g., `TestListItems_Success`, `TestCreateItem_EmptyName`
- **Layers tested independently**: service tests mock repository port, gRPC adapter tests mock service port

### BDD / Cucumber Tests

- Gherkin features in `adapter/grpc/features/*.feature`
- Step definitions in `item_server_cucumber_test.go` using [godog](https://github.com/cucumber/godog)
- Shared `testContext` struct carries state across Given/When/Then steps
- Tests the gRPC adapter with mocked service — no DB or network

## Logging

Structured JSON via `log/slog`. Tag loggers with component name on construction.
