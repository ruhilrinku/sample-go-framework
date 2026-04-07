package composite

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sample-go/item-service/internal/fds/core/port"
)

// FDSCacheRepository is a composite driven adapter that implements
// port.PlatformFdsIdentifierMapRepository with a local-first lookup strategy:
//
//  1. Query the local postgres mapping table.
//  2. On cache miss (record not found), call the remote FDS gRPC client.
//  3. Store the resolved identifiers back to postgres so the next call is served locally.
type FDSCacheRepository struct {
	local  port.PlatformFdsIdentifierMapService    // postgres adapter
	remote port.PlatformFdsIdentifierMapRepository // FDS gRPC client adapter
	logger *slog.Logger
}

// NewFDSCacheRepository returns a FDSCacheRepository that resolves platform
// identifiers from the local store first and falls back to the remote FDS
// service only when no local mapping exists.
func NewFDSCacheRepository(
	local port.PlatformFdsIdentifierMapService,
	remote port.PlatformFdsIdentifierMapRepository,
	logger *slog.Logger,
) *FDSCacheRepository {
	return &FDSCacheRepository{
		local:  local,
		remote: remote,
		logger: logger.With("component", "fds-cache-repository"),
	}
}

// GetPlatformDetailsbyFDSIdentifiers first checks the local postgres mapping.
// On a cache miss it delegates to the remote FDS gRPC service and persists the
// result so subsequent calls are served from the local table.
func (r *FDSCacheRepository) GetPlatformDetailsbyFDSIdentifiers(ctx context.Context, fdsTenantID, fdsUserID string) (uuid.UUID, uuid.UUID, error) {
	platformTenantID, platformUserID, err := r.local.GetPlatformDetailsbyFDSIdentifiers(ctx, fdsTenantID, fdsUserID)
	if err == nil {
		r.logger.DebugContext(ctx, "FDS identifiers resolved from local cache",
			"fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID)
		return platformTenantID, platformUserID, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		// Unexpected db error — surface it immediately, don't fall through.
		return uuid.Nil, uuid.Nil, err
	}

	r.logger.InfoContext(ctx, "FDS mapping not found locally — resolving via remote FDS service",
		"fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID)

	platformTenantID, platformUserID, err = r.remote.GetPlatformDetailsbyFDSIdentifiers(ctx, fdsTenantID, fdsUserID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	// Cache the resolved mapping locally so future calls skip the remote hop.
	if cacheErr := r.local.CreatePlatformFdsIdentifierMapping(ctx, fdsTenantID, fdsUserID, platformTenantID, platformUserID); cacheErr != nil {
		// Log but don't fail — the resolved IDs are valid even if caching failed.
		r.logger.WarnContext(ctx, "failed to cache FDS mapping locally",
			"fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID, "error", cacheErr)
	}

	return platformTenantID, platformUserID, nil
}

// CreatePlatformFdsIdentifierMapping delegates directly to the local postgres adapter.
func (r *FDSCacheRepository) CreatePlatformFdsIdentifierMapping(ctx context.Context, fdsTenantID, fdsUserID string, platformTenantID, platformUserID uuid.UUID) error {
	return r.local.CreatePlatformFdsIdentifierMapping(ctx, fdsTenantID, fdsUserID, platformTenantID, platformUserID)
}
