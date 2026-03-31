# Item Service — gRPC Microservice (Hexagonal Architecture)

A Go gRPC microservice built with **hexagonal (ports & adapters) architecture**, backed by **PostgreSQL** with **GORM**, **UUID v7** identifiers, structured logging, reader/writer database separation, and **gRPC-JSON REST transcoding** via grpc-gateway.

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

## Project Structure

```
├── cmd/server/                         # Application entry point (main.go)
├── proto/
│   ├── item/v1/                        # Protobuf service definitions (with HTTP annotations)
│   └── google/api/                     # Vendored googleapis proto (annotations, http)
├── gen/pb/item/v1/                     # Generated Go protobuf/gRPC/gateway code
├── internal/
│   ├── config/                         # Configuration (app.properties) & BaseModel
│   ├── core/
│   │   ├── domain/                     # Domain models & custom error types
│   │   ├── port/                       # Port interfaces (service & repository)
│   │   └── service/                    # Application service (business logic)
│   └── adapter/
│       ├── driving/grpc/               # gRPC server adapter (primary)
│       └── driven/
│           ├── postgres/               # PostgreSQL repository, data model, converter
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
| `name`       | VARCHAR(255) | Item name                      |
| `description`| TEXT         | Item description               |
| `created_at` | TIMESTAMP    | Auto-set on creation           |
| `modified_at`| TIMESTAMP    | Set on modification (NULL on create) |
| `created_by` | VARCHAR(255) | Audit field (default: "System")|
| `modified_by`| VARCHAR(255) | Audit field (NULL on create)   |
| `is_deleted` | BOOLEAN      | Soft delete flag               |

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

**REST example:**
```bash
curl -X POST http://localhost:8080/api/v1/items \
  -H "Content-Type: application/json" \
  -d '{"name": "Widget", "description": "A sample widget"}'
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
curl http://localhost:8080/api/v1/items?page=1&page_size=10
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

The project includes unit tests with mocked dependencies:

- **Service layer tests** (`internal/core/service/item_service_test.go`) — 7 tests covering ListItems (success, error, page defaults, page cap) and CreateItem (success, empty name validation, repository error)
- **gRPC adapter tests** (`internal/adapter/driving/grpc/item_server_test.go`) — 8 tests covering ListItems (success, error, empty result, not found) and CreateItem (success, empty name, service error, validation error)

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
