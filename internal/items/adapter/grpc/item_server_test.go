package grpc_test

import (
	"context"
	"log/slog"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	itemv1 "github.com/sample-go/item-service/gen/pb/item/v1"
	grpcadapter "github.com/sample-go/item-service/internal/items/adapter/grpc"
	"github.com/sample-go/item-service/internal/items/core/domain"
)

// mockItemService is a test double for port.ItemService.
type mockItemService struct {
	items       []domain.ItemDomainModel
	total       int
	err         error
	createdItem domain.ItemDomainModel
	createErr   error
}

func (m *mockItemService) ListItems(_ context.Context, _, _ int) ([]domain.ItemDomainModel, int, error) {
	return m.items, m.total, m.err
}

func (m *mockItemService) CreateItem(_ context.Context, _, _ string) (domain.ItemDomainModel, error) {
	return m.createdItem, m.createErr
}

func TestItemServer_ListItems_Success(t *testing.T) {
	items := []domain.ItemDomainModel{
		{ID: "a1", Name: "Alpha", Description: "First"},
		{ID: "b2", Name: "Beta", Description: "Second"},
	}

	svc := &mockItemService{items: items, total: 2}
	server := grpcadapter.NewItemServer(svc, slog.Default())

	resp, err := server.ListItems(context.Background(), &itemv1.ListItemsRequest{
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", resp.TotalCount)
	}
	if len(resp.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(resp.Items))
	}
	if resp.Items[0].Id != "a1" {
		t.Errorf("Items[0].Id = %q, want %q", resp.Items[0].Id, "a1")
	}
	if resp.Items[0].Name != "Alpha" {
		t.Errorf("Items[0].Name = %q, want %q", resp.Items[0].Name, "Alpha")
	}
}

func TestItemServer_ListItems_ServiceError(t *testing.T) {
	svc := &mockItemService{err: domain.NewInternalError("db failure", nil)}
	server := grpcadapter.NewItemServer(svc, slog.Default())

	_, err := server.ListItems(context.Background(), &itemv1.ListItemsRequest{
		Page:     1,
		PageSize: 10,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T", err)
	}
	if st.Code() != codes.Internal {
		t.Errorf("code = %v, want %v", st.Code(), codes.Internal)
	}
}

func TestItemServer_ListItems_EmptyResult(t *testing.T) {
	svc := &mockItemService{items: nil, total: 0}
	server := grpcadapter.NewItemServer(svc, slog.Default())

	resp, err := server.ListItems(context.Background(), &itemv1.ListItemsRequest{
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", resp.TotalCount)
	}
	if len(resp.Items) != 0 {
		t.Errorf("len(Items) = %d, want 0", len(resp.Items))
	}
}

func TestItemServer_CreateItem_Success(t *testing.T) {
	svc := &mockItemService{
		createdItem: domain.ItemDomainModel{
			ID:          "c1",
			Name:        "New Item",
			Description: "Desc",
		},
	}
	server := grpcadapter.NewItemServer(svc, slog.Default())

	resp, err := server.CreateItem(context.Background(), &itemv1.CreateItemRequest{
		Name:        "New Item",
		Description: "Desc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Item.Id != "c1" {
		t.Errorf("Item.Id = %q, want %q", resp.Item.Id, "c1")
	}
	if resp.Item.Name != "New Item" {
		t.Errorf("Item.Name = %q, want %q", resp.Item.Name, "New Item")
	}
	if resp.Item.Description != "Desc" {
		t.Errorf("Item.Description = %q, want %q", resp.Item.Description, "Desc")
	}
}

func TestItemServer_CreateItem_EmptyName(t *testing.T) {
	svc := &mockItemService{}
	server := grpcadapter.NewItemServer(svc, slog.Default())

	_, err := server.CreateItem(context.Background(), &itemv1.CreateItemRequest{
		Name: "",
	})
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
}

func TestItemServer_CreateItem_ServiceError(t *testing.T) {
	svc := &mockItemService{createErr: domain.NewInternalError("db failure", nil)}
	server := grpcadapter.NewItemServer(svc, slog.Default())

	_, err := server.CreateItem(context.Background(), &itemv1.CreateItemRequest{
		Name:        "Item",
		Description: "Desc",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T", err)
	}
	if st.Code() != codes.Internal {
		t.Errorf("code = %v, want %v", st.Code(), codes.Internal)
	}
}

func TestItemServer_CreateItem_ValidationError(t *testing.T) {
	svc := &mockItemService{createErr: domain.NewValidationError("name too long")}
	server := grpcadapter.NewItemServer(svc, slog.Default())

	_, err := server.CreateItem(context.Background(), &itemv1.CreateItemRequest{
		Name:        "Item",
		Description: "Desc",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
}

func TestItemServer_ListItems_NotFoundError(t *testing.T) {
	svc := &mockItemService{err: domain.NewNotFoundError("items not found")}
	server := grpcadapter.NewItemServer(svc, slog.Default())

	_, err := server.ListItems(context.Background(), &itemv1.ListItemsRequest{
		Page:     1,
		PageSize: 10,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("code = %v, want %v", st.Code(), codes.NotFound)
	}
}
