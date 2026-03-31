# Item Service — gRPC Microservice (Hexagonal Architecture)

A Go gRPC microservice using hexagonal (ports & adapters) architecture with PostgreSQL.

## Project Structure

```
├── cmd/server/              # Application entry point
├── proto/item/v1/           # Protobuf definitions
├── gen/pb/item/v1/          # Generated Go protobuf code
├── internal/
│   ├── config/              # Configuration loading
│   ├── core/
│   │   ├── domain/          # Domain entities
│   │   ├── port/            # Port interfaces (primary & secondary)
│   │   └── service/         # Application service (use cases)
│   └── adapter/
│       ├── driving/grpc/    # gRPC server (primary/driving adapter)
│       └── driven/postgres/ # PostgreSQL repository (secondary/driven adapter)
├── migrations/              # SQL migrations
├── buf.yaml                 # Buf module config
├── buf.gen.yaml             # Buf code generation config
└── Makefile
```

## Prerequisites

- Go 1.21+
- PostgreSQL

## Setup

1. **Create the database and run migrations:**
   ```sql
   CREATE DATABASE item_service;
   -- then run:
   psql -d item_service -f migrations/001_create_items.sql
   ```

2. **Set environment variables:**
   ```bash
   export DATABASE_URL="postgres://user:password@localhost:5432/item_service?sslmode=disable"
   export GRPC_PORT=50051  # optional, defaults to 50051
   ```

3. **Run the server:**
   ```bash
   make run
   ```

## Development

```bash
make generate   # Regenerate protobuf Go code
make build      # Build all packages
make test       # Run unit tests
```

## API

### `ItemService.ListItems`

Retrieves a paginated list of items.

**Request:**
| Field      | Type  | Description            |
|------------|-------|------------------------|
| page       | int32 | Page number (1-based)  |
| page_size  | int32 | Items per page (max 100) |

**Response:**
| Field       | Type      | Description          |
|-------------|-----------|----------------------|
| items       | []Item    | List of items        |
| total_count | int32     | Total items in DB    |
