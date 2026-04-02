# Item Service — gRPC Microservice (Hexagonal Architecture)

A Go gRPC microservice built with **hexagonal (ports & adapters) architecture**, backed by **PostgreSQL** with **GORM**, **UUID v7** identifiers, structured logging, reader/writer database separation, **gRPC-JSON REST transcoding** via grpc-gateway, **FDS IAM JWT authentication**, **distributed trace propagation**, and **column-level multi-tenancy** with automatic tenant scoping.

## Tech Stack

| Component            | Technology                                      |
|----------------------|-------------------------------------------------|
| Language             | Go 1.26+                                        |
| Transport            | gRPC (`google.golang.org/grpc`)                 |
| REST Transcoding     | grpc-gateway v2 (`github.com/grpc-ecosystem/grpc-gateway/v2`) |
| Serialization        | Protocol Buffers (`google.golang.org/protobuf`) |
| ORM                  | GORM (`gorm.io/gorm`) + PostgreSQL driver       |
| Database             | PostgreSQL (via `pgx` for migrations, GORM for queries) |
| ID Generation        | UUID v7 (`github.com/google/uuid`)              |
| Migrations           | Pure Go Liquibase-compatible runner (YAML changelogs) |
| Logging              | `log/slog` (structured JSON)                    |
| BDD Testing          | Cucumber/Godog (`github.com/cucumber/godog`)    |
| Protobuf Generation  | Buf CLI (local plugins)                         |

## Architecture

The project follows **hexagonal architecture** (ports & adapters) to keep the domain logic decoupled from infrastructure:

```
              ┌────────────────────────────────────────────────┐
              │                  Domain Core                    │
              │  ┌────────────────────────────────────────┐    │
  HTTP REST ──┤  │  Ports (interfaces)                    │    ├── PostgreSQL Adapter
  (gateway)   │  │    ├─ ItemService (primary)             │    │   (driven)
              │  │    └─ ItemRepository (secondary)        │    │
  gRPC ───────┤  ├────────────────────────────────────────┤    │
  (driving)   │  │  Service (business logic)               │    │
              │  │  Domain Models & Errors                 │    │
              │  └────────────────────────────────────────┘    │
              └────────────────────────────────────────────────┘
```

The HTTP REST gateway (grpc-gateway) acts as a reverse proxy, translating JSON/HTTP requests into gRPC calls and forwarding them to the gRPC server.

Key design decisions:
- **gRPC-JSON REST transcoding** — grpc-gateway v2 reverse proxy exposes all gRPC APIs as REST/JSON endpoints automatically from proto HTTP annotations
- **Separate domain and data models** — `ItemDomainModel` (clean, no metadata) and `ItemDataModel` (GORM-tagged, with audit fields) with a converter layer
- **Reader/Writer DB separation** — Separate GORM connections for read (replica) and write (primary) operations
- **Custom domain errors** — Typed errors (`Validation`, `NotFound`, `Internal`) mapped to gRPC status codes
- **UUID v7** — Time-ordered UUIDs generated in the application layer via GORM `BeforeCreate` hook
- **FDS IAM JWT interceptor** — Global gRPC interceptor authenticates via `Authorization: Bearer <token>` (FDS-issued or standard JWT), falls back to explicit `x-tenant-id` / `x-user-id` headers, populates a typed `RequestSession` on the context, and propagates to all downstream layers for both gRPC and REST calls
- **Distributed trace propagation** — Trace headers (`x-b3-traceid`, `x-b3-spanid`, `x-request-id`, `x-correlation-id`, etc.) are extracted into `RequestSession.Traces` on every request
- **Column-level multi-tenancy** — Every query is automatically scoped by `tenant_id` via a GORM scope function (`tenant.Scope(ctx)`), enforcing tenant isolation at the data layer

## Project Structure

```
├── cmd/server/                         # Application entry point (main.go)
├── proto/
│   ├── item/v1/                        # Protobuf service definitions (with HTTP annotations)
│   └── google/api/                     # Vendored googleapis proto (annotations, http)
├── gen/pb/item/v1/                     # Generated Go protobuf/gRPC/gateway code
├── internal/
│   ├── config/                         # Configuration (app.properties) & BaseModel
│   ├── session/                        # Request session model, interceptor & header matcher
│   ├── core/
│   │   ├── domain/                     # Domain models & custom error types
│   │   ├── port/                       # Port interfaces (service & repository)
│   │   └── service/                    # Application service (business logic)
│   └── adapter/
│       ├── driving/grpc/               # gRPC server adapter (primary)
│       │   └── features/               # Cucumber/Gherkin feature files
│       └── driven/
│           ├── postgres/               # PostgreSQL repository, data model, converter
│           │   └── tenant/             # Tenant scoping (GORM scope & tenant ID setter)
│           └── liquibase/              # Pure Go migration runner
├── db/
│   ├── changelog-master.yaml           # Liquibase master changelog
│   └── changelogs/                     # Individual migration changesets
├── app.properties                      # Application configuration
├── buf.yaml                            # Buf module config
├── buf.gen.yaml                        # Buf code generation config
├── Makefile                            # Build, test, generate commands
└── .vscode/launch.json                 # Debug configuration (Delve)
```

## Prerequisites

- **Go 1.26+**
- **PostgreSQL** (running instance)
- **buf CLI** (for protobuf code generation, installed automatically via `go run`)

## Configuration

Configuration is loaded from `app.properties` with environment variable overrides (uppercase, e.g. `DATABASE_URL`):

| Property               | Default                                              | Description                      |
|------------------------|------------------------------------------------------|----------------------------------|
| `database_url`         | `postgres://postgres:password@localhost:5432/item_service?sslmode=disable` | Primary DB connection (migrations) |
| `database_reader_url`  | Falls back to `database_url`                         | Read replica connection (GORM)   |
| `database_writer_url`  | Falls back to `database_url`                         | Write primary connection (GORM)  |
| `grpc_port`            | `50051`                                              | gRPC server port                 |
| `http_port`            | `8080`                                               | HTTP REST gateway port           |
| `liquibase_changelog`  | `db/changelog-master.yaml`                           | Path to migration changelog      |
| `fds_issuer`           | *(empty)*                                            | JWT `iss` claim value for FDS-issued tokens; enables FDS identity resolution when set |

## Getting Started

1. **Create the database:**
   ```sql
   CREATE DATABASE item_service;
   ```

2. **Configure connection** (edit `app.properties` or set environment variables):
   ```bash
   export DATABASE_URL="postgres://user:password@localhost:5432/item_service?sslmode=disable"
   ```

3. **Run the server** (migrations run automatically on startup):
   ```bash
   make run
   ```

## Development

```bash
make generate   # Regenerate protobuf Go code (uses buf CLI)
make build      # Build all packages
make test       # Run unit tests with verbose output
make clean      # Remove generated protobuf code
```

## Database Migrations

Migrations use a **pure Go Liquibase-compatible runner** — no external Liquibase installation needed. Changelogs are defined in YAML under `db/`:

- `db/changelog-master.yaml` — Master changelog (includes individual changesets)
- `db/changelogs/001-create-items-table.yaml` — Creates `items` table and index

Migrations run **automatically on server startup** via `pgx`. A `databasechangelog` tracking table is created to manage applied changesets and checksums.

### Items Table Schema

| Column       | Type         | Description                    |
|--------------|--------------|--------------------------------|
| `id`         | UUID (PK)    | UUID v7, generated in app layer |
| `tenant_id`  | UUID         | Tenant identifier (NOT NULL)   |
| `name`       | VARCHAR(255) | Item name                      |
| `description`| TEXT         | Item description               |
| `created_at` | TIMESTAMPTZ  | Auto-set on creation           |
| `modified_at`| TIMESTAMPTZ  | Set on modification (NULL on create) |
| `created_by` | UUID         | User ID from session context   |
| `modified_by`| UUID         | User ID on modification (NULL on create) |
| `is_deleted` | BOOLEAN      | Soft delete flag               |

Indexes: `idx_items_tenant_id` (tenant_id), `idx_items_created_at` (created_at)

## Request Session

Every inbound call (gRPC or REST) passes through the `session.UnaryInterceptor`. It builds a `RequestSession` and stores it on the Go context — accessible anywhere via `session.FromContext(ctx)`.

### Authentication modes

The interceptor supports two authentication modes, checked in order:

**1. JWT Bearer token** (`Authorization: Bearer <token>`)

The JWT payload is base64url-decoded and identity claims are mapped to the session. The `iss` claim is compared against `fds_issuer`:

| Claim | Session field | Notes |
|---|---|---|
| `tenant_id` | `TenantID` (uuid.UUID) | Required, must be a valid UUID |
| `user_id` | `UserID` (uuid.UUID) | Required, must be a valid UUID |
| `email` | `Email` | Optional |
| `culture_code` | `CultureCode` | Optional |
| `iss` == `fds_issuer` | `FDSClaims` | Populated only for FDS-issued tokens (see below) |

> **JWT signature verification** is not performed in-process. Add JWKS-based signature validation against the issuer's public keys before deploying to production.

**2. Explicit identity headers** (fallback when no Bearer token is present)

| Header | Type | Required |
|---|---|---|
| `x-tenant-id` | UUID | Yes |
| `x-user-id` | UUID | Yes |

Missing or empty values → `UNAUTHENTICATED`. Malformed (non-UUID) values → `INVALID_ARGUMENT`.

### FDS IAM tokens

When `fds_issuer` is configured and an incoming JWT's `iss` claim matches, the interceptor treats it as a **Federated Data System (FDS)** token. The raw FDS-native identifiers are captured in `RequestSession.FDSClaims` for downstream platform identity resolution:

```go
type FDSClaims struct {
    TenantID  string // fds_tenant_id JWT claim
    UserID    string // fds_user_id JWT claim
    UserEmail string // email JWT claim
}
```

Downstream handlers that need to resolve FDS identifiers to platform UUIDs should check `sess.FDSClaims != nil` and call the appropriate identity resolution service.

### Distributed tracing

Trace headers are extracted from every request into `RequestSession.Traces` regardless of authentication mode:

| Header | Field |
|---|---|
| `x-b3-traceid` | `Traces.TraceID` |
| `x-b3-spanid` | `Traces.SpanID` |
| `x-b3-parentspanid` | `Traces.ParentSpanID` |
| `x-request-id` | `Traces.RequestID` |
| `x-correlation-id` | `Traces.CorrelationID` |

All these headers are also forwarded from the REST gateway into gRPC metadata without the `grpcgateway-` prefix via `session.GatewayHeaderMatcher`.

### RequestSession reference

```go
type RequestSession struct {
    TenantID    uuid.UUID  // populated from JWT claim or x-tenant-id header
    UserID      uuid.UUID  // populated from JWT claim or x-user-id header
    Email       string     // populated from JWT email claim
    CultureCode string     // populated from JWT culture_code claim
    Traces      Traces     // distributed trace identifiers
    FDSClaims   *FDSClaims // non-nil only when JWT iss == fds_issuer
}
```

The `created_by` audit field on new items is automatically populated from the session's `UserID` UUID.

### Multi-Tenancy

Tenant isolation is enforced at the data layer via automatic GORM scoping:

- **Reads** — `tenant.Scope(ctx)` appends `WHERE tenant_id = ?` to every query
- **Writes** — `tenant.SetTenantID(ctx)` extracts the tenant UUID from the session and populates the `BaseModel.TenantID` field before insert

All repository operations include tenant scoping. No data from other tenants is ever visible.

## API

All APIs are available via both **gRPC** (port `50051`) and **REST/JSON** (port `8080`). The REST endpoints are auto-generated from the protobuf HTTP annotations using grpc-gateway.

### CreateItem

Creates a new item. The ID (UUID v7) and metadata fields are generated server-side.

| | gRPC | REST |
|---|---|---|
| **Method** | `ItemService.CreateItem` | `POST /api/v1/items` |

**Request:**
| Field       | Type   | Description          |
|-------------|--------|----------------------|
| name        | string | Item name (required) |
| description | string | Item description     |

**REST example (explicit headers):**
```bash
curl -X POST http://localhost:8080/api/v1/items \
  -H "Content-Type: application/json" \
  -H "x-tenant-id: 10000000-0000-0000-0000-000000000001" \
  -H "x-user-id: 20000000-0000-0000-0000-000000000002" \
  -d '{"name": "Widget", "description": "A sample widget"}'
```

**REST example (JWT Bearer):**
```bash
curl -X POST http://localhost:8080/api/v1/items \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-jwt-token>" \
  -d '{"name": "Widget", "description": "A sample widget"}'
```
```

**Response:**
| Field | Type | Description    |
|-------|------|----------------|
| item  | Item | Created item   |

### ListItems

Retrieves a paginated list of items.

| | gRPC | REST |
|---|---|---|
| **Method** | `ItemService.ListItems` | `GET /api/v1/items` |

**Request:**
| Field      | Type  | Description                     |
|------------|-------|---------------------------------|
| page       | int32 | Page number (1-based, default 1)|
| page_size  | int32 | Items per page (1–100, default 10) |

**REST example:**
```bash
curl -H "x-tenant-id: 10000000-0000-0000-0000-000000000001" \
  -H "x-user-id: 20000000-0000-0000-0000-000000000002" \
  "http://localhost:8080/api/v1/items?page=1&page_size=10"
```

**Response:**
| Field       | Type      | Description          |
|-------------|-----------|----------------------|
| items       | []Item    | List of items        |
| total_count | int32     | Total items in DB    |

### Item Message

| Field       | Type   | Description          |
|-------------|--------|----------------------|
| id          | string | UUID v7 identifier   |
| name        | string | Item name            |
| description | string | Item description     |

### Error Handling

Domain errors are mapped to gRPC status codes:

| Domain Error Type | gRPC Code            | Example                  |
|-------------------|----------------------|--------------------------|
| Validation        | `INVALID_ARGUMENT`   | Empty item name          |
| NotFound          | `NOT_FOUND`          | Item not found           |
| Internal          | `INTERNAL`           | Database failure         |

A **panic recovery interceptor** catches unhandled panics and returns `INTERNAL` with a stack trace logged.

## Testing

```bash
make test
```

The project includes unit tests and BDD (Cucumber) narrow integration tests with mocked dependencies:

### Unit Tests

- **Service layer tests** (`internal/core/service/item_service_test.go`) — 7 tests covering ListItems (success, error, page defaults, page cap) and CreateItem (success, empty name validation, repository error)
- **gRPC adapter tests** (`internal/adapter/driving/grpc/item_server_test.go`) — 8 tests covering ListItems (success, error, empty result, not found) and CreateItem (success, empty name, service error, validation error)
- **Session interceptor tests** (`internal/session/interceptor_test.go`) — 15 tests across three groups:
  - *Header auth:* success, missing metadata, missing tenant, missing user, empty values, invalid tenant UUID, invalid user UUID
  - *JWT auth:* success, FDS issuer match populates `FDSClaims`, FDS issuer mismatch, malformed token, invalid UUID in claims
  - *Traces:* headers populate all trace fields, absent headers yield empty traces
  - *Context helpers:* nil when no session, round-trip
- **Tenant scope tests** (`internal/adapter/driven/postgres/tenant/scope_test.go`) — 4 tests covering SetTenantID success, no session, empty tenant ID, and Scope with no session

### Cucumber / BDD Tests (Narrow Integration)

Gherkin feature files under `internal/adapter/driving/grpc/features/` define behavior scenarios executed via [godog](https://github.com/cucumber/godog). These test the gRPC adapter layer with a mocked service port — verifying request/response mapping and error code translation without a database or network.

- **CreateItem scenarios** (`features/create_item.feature`) — 4 scenarios: success, empty name validation, internal error, validation error
- **ListItems scenarios** (`features/list_items.feature`) — 4 scenarios: success with data table, empty result, internal error, not found error

Run only the cucumber tests:
```bash
go test ./internal/adapter/driving/grpc/... -run TestFeatures -v
```

## Debugging

A VS Code launch configuration is provided in `.vscode/launch.json`:

- **Launch Server** — Starts the server with Delve debugger
- **Debug Tests** — Runs tests under debugger

## Logging

Structured JSON logging via `log/slog` across all layers:

```json
{"time":"...","level":"INFO","msg":"gRPC server listening","address":":50051"}
{"time":"...","level":"INFO","msg":"HTTP gateway listening","address":":8080"}
{"time":"...","level":"INFO","msg":"item created","id":"...","name":"Widget"}
{"time":"...","level":"ERROR","msg":"failed to create item","error":"..."}
```
