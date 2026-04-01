package tenant

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sample-go/item-service/internal/session"
)

// Scope returns a GORM scope function that filters queries by the tenant ID
// extracted from the request session in the context. This should be applied
// to every database query to enforce tenant isolation.
//
// Usage:
//
//	db.WithContext(ctx).Scopes(tenant.Scope(ctx)).Find(&items)
//	db.WithContext(ctx).Scopes(tenant.Scope(ctx)).Create(&item)
func Scope(ctx context.Context) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		sess := session.FromContext(ctx)
		if sess == nil || sess.TenantID == uuid.Nil {
			_ = db.AddError(fmt.Errorf("tenant context is required: no tenant ID in session"))
			return db
		}
		return db.Where("tenant_id = ?", sess.TenantID)
	}
}

// SetTenantID extracts the tenant ID from the context and returns it.
// Use this when populating the TenantID field on data models before insert.
// Returns an error if no session or tenant ID is present.
func SetTenantID(ctx context.Context) (uuid.UUID, error) {
	sess := session.FromContext(ctx)
	if sess == nil || sess.TenantID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("tenant context is required: no tenant ID in session")
	}
	return sess.TenantID, nil
}
