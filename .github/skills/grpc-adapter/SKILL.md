---
name: grpc-adapter
description: "Use when: creating a gRPC server adapter, implementing RPC handlers, adding protobuf-to-domain mapping, translating domain errors to gRPC status codes. Covers gRPC server struct, handler methods, error translation via toGRPCError, protobuf field mapping, and grpc-gateway HTTP annotations."
argument-hint: "Provide the feature name and RPC operations to implement"
---

# gRPC Adapter

## When to Use

- Creating a new gRPC server adapter for a feature
- Adding new RPC handler methods
- User says "add gRPC handler", "implement RPC", "add API endpoint"

## Architecture

gRPC adapter files live in `internal/<feature>/adapter/grpc/`:
```
<feature>_server.go           # Server struct + RPC handlers + error translation
<feature>_server_test.go      # Unit tests (mock service port)
```

The adapter depends on:
- Generated protobuf code from `gen/pb/<feature>/v1/`
- Service port interface from `internal/<feature>/core/port/`
- Domain error types from `internal/<feature>/core/domain/` (or shared `internal/items/core/domain/`)

## Procedure

### 1. Define Protobuf

File: `proto/<feature>/v1/<feature>.proto`

```protobuf
syntax = "proto3";

package <feature>.v1;

import "google/api/annotations.proto";

option go_package = "github.com/sample-go/item-service/gen/pb/<feature>/v1;<feature>v1";

service <Feature>Service {
  rpc List<Feature>s(List<Feature>sRequest) returns (List<Feature>sResponse) {
    option (google.api.http) = {
      get: "/api/v1/<features>"
    };
  }
  rpc Create<Feature>(Create<Feature>Request) returns (Create<Feature>Response) {
    option (google.api.http) = {
      post: "/api/v1/<features>"
      body: "*"
    };
  }
}

message Create<Feature>Request {
  string name = 1;
  string description = 2;
}

message Create<Feature>Response {
  <Feature> <feature> = 1;
}

message List<Feature>sRequest {
  int32 page_size = 1;
  int32 page = 2;
}

message List<Feature>sResponse {
  repeated <Feature> <features> = 1;
  int32 total_count = 2;
}

message <Feature> {
  string id = 1;
  string name = 2;
  string description = 3;
}
```

Rules:
- HTTP annotations required for grpc-gateway
- GET for list/read, POST with `body: "*"` for create
- Run `make generate` after editing proto files
- Never manually edit files in `gen/pb/`

### 2. Create Server Struct

File: `internal/<feature>/adapter/grpc/<feature>_server.go`

```go
package grpc

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/sample-go/item-service/gen/pb/<feature>/v1"
	"github.com/sample-go/item-service/internal/<feature>/core/domain"
	"github.com/sample-go/item-service/internal/<feature>/core/port"
)

type <Feature>Server struct {
	pb.Unimplemented<Feature>ServiceServer
	svc    port.<Feature>Service
	logger *slog.Logger
}

func New<Feature>Server(svc port.<Feature>Service, logger *slog.Logger) *<Feature>Server {
	return &<Feature>Server{
		svc:    svc,
		logger: logger.With("component", "grpc-server"),
	}
}
```

Rules:
- Always embed `Unimplemented<Feature>ServiceServer` for forward-compatible gRPC
- Constructor accepts the **service port interface**, not concrete service
- Logger tagged with `"component", "grpc-server"`

### 3. Implement RPC Handlers

#### List Handler
```go
func (s *<Feature>Server) List<Feature>s(ctx context.Context, req *pb.List<Feature>sRequest) (*pb.List<Feature>sResponse, error) {
	page := int(req.GetPage())
	pageSize := int(req.GetPageSize())

	s.logger.InfoContext(ctx, "List<Feature>s called", "page", page, "pageSize", pageSize)

	items, total, err := s.svc.List<Feature>s(ctx, page, pageSize)
	if err != nil {
		s.logger.ErrorContext(ctx, "List<Feature>s failed", "error", err)
		return nil, toGRPCError(err)
	}

	pbItems := make([]*pb.<Feature>, 0, len(items))
	for _, item := range items {
		pbItems = append(pbItems, &pb.<Feature>{
			Id:          item.ID,
			Name:        item.Name,
			Description: item.Description,
		})
	}

	return &pb.List<Feature>sResponse{
		<Feature>s:   pbItems,
		TotalCount: int32(total),
	}, nil
}
```

#### Create Handler
```go
func (s *<Feature>Server) Create<Feature>(ctx context.Context, req *pb.Create<Feature>Request) (*pb.Create<Feature>Response, error) {
	name := req.GetName()
	if name == "" {
		s.logger.WarnContext(ctx, "Create<Feature> called with empty name")
		return nil, status.Errorf(codes.InvalidArgument, "name is required")
	}

	s.logger.InfoContext(ctx, "Create<Feature> called", "name", name)

	item, err := s.svc.Create<Feature>(ctx, name, req.GetDescription())
	if err != nil {
		s.logger.ErrorContext(ctx, "Create<Feature> failed", "name", name, "error", err)
		return nil, toGRPCError(err)
	}

	return &pb.Create<Feature>Response{
		<Feature>: &pb.<Feature>{
			Id:          item.ID,
			Name:        item.Name,
			Description: item.Description,
		},
	}, nil
}
```

### 4. Error Translation

Every gRPC adapter MUST include this helper:
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

Error mapping table:

| Domain Error | gRPC Code | HTTP Status (via gateway) |
|---|---|---|
| `ErrorTypeValidation` | `InvalidArgument` (3) | 400 Bad Request |
| `ErrorTypeNotFound` | `NotFound` (5) | 404 Not Found |
| `ErrorTypeInternal` | `Internal` (13) | 500 Internal Server Error |

### 5. Key Boundaries

| Rule | Details |
|---|---|
| **Protobuf at boundary only** | Protobuf types never leak into service or domain |
| **Domain models in service calls** | Service port accepts/returns domain types only |
| **Adapter owns translation** | All protobuf ↔ domain conversion happens here |
| **Input validation at adapter** | Basic field presence checks before calling service |
| **Error translation at boundary** | `toGRPCError()` converts all domain errors to gRPC status |
| **Structured logging** | `InfoContext` on entry/exit, `ErrorContext` on failure, `WarnContext` on bad input |

### 6. Register in main.go

```go
// In cmd/server/main.go
<feature>v1.Register<Feature>ServiceServer(grpcServer, <feature>Server)

// Register gateway
<feature>v1.Register<Feature>ServiceHandlerFromEndpoint(ctx, gwMux, grpcAddr, opts)
```
