package port

import (
	"context"

	"github.com/google/uuid"
)

type PlatformFdsIdentifierMapService interface {
	GetPlatformDetailsbyFDSIdentifiers(ctx context.Context, fdsTenantID, fdsUserID string) (uuid.UUID, uuid.UUID, error)
	CreatePlatformFdsIdentifierMapping(ctx context.Context, fdsTenantID, fdsUserID string, platformTenantID, platformUserID uuid.UUID) error
}
