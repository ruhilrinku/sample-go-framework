package tenant_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sample-go/item-service/config/postgres/tenant"
	"github.com/sample-go/item-service/config/session"
)

var (
	testTenantID = uuid.MustParse("10000000-0000-0000-0000-000000000001")
	testUserID   = uuid.MustParse("20000000-0000-0000-0000-000000000002")
)

func TestSetTenantID_Success(t *testing.T) {
	ctx := session.WithSession(context.Background(), &session.RequestSession{
		TenantID: testTenantID,
		UserID:   testUserID,
	})

	id, err := tenant.SetTenantID(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != testTenantID {
		t.Errorf("TenantID = %q, want %q", id, testTenantID)
	}
}

func TestSetTenantID_NoSession(t *testing.T) {
	_, err := tenant.SetTenantID(context.Background())
	if err == nil {
		t.Fatal("expected error for missing session, got nil")
	}
}

func TestSetTenantID_EmptyTenantID(t *testing.T) {
	ctx := session.WithSession(context.Background(), &session.RequestSession{
		TenantID: uuid.Nil,
		UserID:   testUserID,
	})

	_, err := tenant.SetTenantID(ctx)
	if err == nil {
		t.Fatal("expected error for empty tenant ID, got nil")
	}
}

func TestScope_NoSession_AddsError(t *testing.T) {
	// We can't easily test GORM scope without a real DB, but we can verify
	// the Scope function doesn't panic when called without a session.
	scopeFn := tenant.Scope(context.Background())
	if scopeFn == nil {
		t.Fatal("expected scope function, got nil")
	}
}
