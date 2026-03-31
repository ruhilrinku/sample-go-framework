package grpc

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	itemv1 "github.com/sample-go/item-service/gen/pb/item/v1"
	"github.com/sample-go/item-service/internal/core/domain"
	"github.com/sample-go/item-service/internal/core/port"
)

// ItemServer implements the gRPC ItemServiceServer.
type ItemServer struct {
	itemv1.UnimplementedItemServiceServer
	svc    port.ItemService
	logger *slog.Logger
}

// NewItemServer creates a new gRPC handler backed by the given service.
func NewItemServer(svc port.ItemService, logger *slog.Logger) *ItemServer {
	return &ItemServer{svc: svc, logger: logger.With("component", "grpc-server")}
}

// ListItems handles the ListItems RPC.
func (s *ItemServer) ListItems(ctx context.Context, req *itemv1.ListItemsRequest) (*itemv1.ListItemsResponse, error) {
	page := int(req.GetPage())
	pageSize := int(req.GetPageSize())

	s.logger.InfoContext(ctx, "ListItems called", "page", page, "pageSize", pageSize)

	items, total, err := s.svc.ListItems(ctx, page, pageSize)
	if err != nil {
		s.logger.ErrorContext(ctx, "ListItems failed", "error", err)
		return nil, toGRPCError(err)
	}

	pbItems := make([]*itemv1.Item, 0, len(items))
	for _, item := range items {
		pbItems = append(pbItems, &itemv1.Item{
			Id:          item.ID,
			Name:        item.Name,
			Description: item.Description,
		})
	}

	s.logger.InfoContext(ctx, "ListItems completed", "count", len(items), "total", total)
	return &itemv1.ListItemsResponse{
		Items:      pbItems,
		TotalCount: int32(total),
	}, nil
}

// CreateItem handles the CreateItem RPC.
func (s *ItemServer) CreateItem(ctx context.Context, req *itemv1.CreateItemRequest) (*itemv1.CreateItemResponse, error) {
	name := req.GetName()
	if name == "" {
		s.logger.WarnContext(ctx, "CreateItem called with empty name")
		return nil, status.Errorf(codes.InvalidArgument, "name is required")
	}

	s.logger.InfoContext(ctx, "CreateItem called", "name", name)

	item, err := s.svc.CreateItem(ctx, name, req.GetDescription())
	if err != nil {
		s.logger.ErrorContext(ctx, "CreateItem failed", "name", name, "error", err)
		return nil, toGRPCError(err)
	}

	s.logger.InfoContext(ctx, "CreateItem completed", "id", item.ID)
	return &itemv1.CreateItemResponse{
		Item: &itemv1.Item{
			Id:          item.ID,
			Name:        item.Name,
			Description: item.Description,
		},
	}, nil
}

// toGRPCError maps a domain error to the appropriate gRPC status error.
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
