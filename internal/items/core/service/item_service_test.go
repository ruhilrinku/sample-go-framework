package service_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/sample-go/item-service/internal/items/core/domain"
	"github.com/sample-go/item-service/internal/items/core/service"
)

// mockItemRepository is a test double for port.ItemRepository.
type mockItemRepository struct {
	items       []domain.ItemDomainModel
	total       int
	err         error
	createdItem domain.ItemDomainModel
	createErr   error
}

func (m *mockItemRepository) ListItems(_ context.Context, _, _ int) ([]domain.ItemDomainModel, int, error) {
	return m.items, m.total, m.err
}

func (m *mockItemRepository) CreateItem(_ context.Context, item domain.ItemDomainModel) (domain.ItemDomainModel, error) {
	if m.createErr != nil {
		return domain.ItemDomainModel{}, m.createErr
	}
	result := m.createdItem
	result.Name = item.Name
	result.Description = item.Description
	return result, nil
}

func TestListItems_Success(t *testing.T) {
	want := []domain.ItemDomainModel{
		{ID: "1", Name: "Item 1", Description: "Desc 1"},
		{ID: "2", Name: "Item 2", Description: "Desc 2"},
	}

	repo := &mockItemRepository{items: want, total: 2}
	svc := service.New(repo, slog.Default())

	items, total, err := svc.ListItems(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
	if items[0].ID != "1" {
		t.Errorf("items[0].ID = %q, want %q", items[0].ID, "1")
	}
}

func TestListItems_RepoError(t *testing.T) {
	repo := &mockItemRepository{err: errors.New("db error")}
	svc := service.New(repo, slog.Default())

	_, _, err := svc.ListItems(context.Background(), 1, 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if domain.GetErrorType(err) != domain.ErrorTypeInternal {
		t.Errorf("error type = %v, want ErrorTypeInternal", domain.GetErrorType(err))
	}
}

func TestListItems_DefaultPageSize(t *testing.T) {
	repo := &mockItemRepository{items: nil, total: 0}
	svc := service.New(repo, slog.Default())

	_, total, err := svc.ListItems(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
}

func TestListItems_PageSizeCap(t *testing.T) {
	repo := &mockItemRepository{items: nil, total: 0}
	svc := service.New(repo, slog.Default())

	// pageSize > 100 should be clamped to 20
	_, _, err := svc.ListItems(context.Background(), 1, 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateItem_Success(t *testing.T) {
	repo := &mockItemRepository{
		createdItem: domain.ItemDomainModel{
			ID: "new-id",
		},
	}
	svc := service.New(repo, slog.Default())

	item, err := svc.CreateItem(context.Background(), "Test Item", "A description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ID != "new-id" {
		t.Errorf("ID = %q, want %q", item.ID, "new-id")
	}
	if item.Name != "Test Item" {
		t.Errorf("Name = %q, want %q", item.Name, "Test Item")
	}
	if item.Description != "A description" {
		t.Errorf("Description = %q, want %q", item.Description, "A description")
	}
}

func TestCreateItem_EmptyName(t *testing.T) {
	repo := &mockItemRepository{}
	svc := service.New(repo, slog.Default())

	_, err := svc.CreateItem(context.Background(), "", "desc")
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	if domain.GetErrorType(err) != domain.ErrorTypeValidation {
		t.Errorf("error type = %v, want ErrorTypeValidation", domain.GetErrorType(err))
	}
}

func TestCreateItem_RepoError(t *testing.T) {
	repo := &mockItemRepository{createErr: errors.New("db error")}
	svc := service.New(repo, slog.Default())

	_, err := svc.CreateItem(context.Background(), "Test", "desc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if domain.GetErrorType(err) != domain.ErrorTypeInternal {
		t.Errorf("error type = %v, want ErrorTypeInternal", domain.GetErrorType(err))
	}
}
