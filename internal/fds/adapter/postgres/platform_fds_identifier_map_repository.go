package postgres

import (
	"log/slog"
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PlatformFdsIdentifierMapRepository struct {
	writerDB *gorm.DB
	readerDB *gorm.DB
	logger   *slog.Logger
}

func NewPlatformFDSIdentifierMappingRepository(writerDB, readerDB *gorm.DB, logger *slog.Logger) *PlatformFdsIdentifierMapRepository {
	return &PlatformFdsIdentifierMapRepository{
		writerDB: writerDB,
		readerDB: readerDB,
		logger:   logger.With("component", "platform-fds-identifier-mapping-repo"),
	}
}

func (r *PlatformFdsIdentifierMapRepository) GetPlatformDetailsbyFDSIdentifiers(ctx context.Context, fdsTenantID, fdsUserID string) (uuid.UUID, uuid.UUID, error) {
	var dataItem PlatformFDSIdentifierMappingDataModel

	r.logger.DebugContext(ctx, "fetching platform identifiers for FDS tenant and user", "fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID)

	if err := r.readerDB.Where("fds_tenant_id = ? AND fds_user_id = ?", fdsTenantID, fdsUserID).First(&dataItem).Error; err != nil {
		r.logger.Error("failed to fetch platform identifiers for FDS tenant and user", "fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID, "error", err)
		return uuid.Nil, uuid.Nil, err
	}

	return dataItem.PlatformTenantID, dataItem.PlatformUserID, nil
}

func (r *PlatformFdsIdentifierMapRepository) CreatePlatformFdsIdentifierMapping(ctx context.Context, fdsTenantID, fdsUserID string, platformTenantID, platformUserID uuid.UUID) error {
	dataItem := PlatformFDSIdentifierMappingDataModel{
		FDSTenantID:      fdsTenantID,
		FDSUserID:        fdsUserID,
		PlatformTenantID: platformTenantID,
		PlatformUserID:   platformUserID,
	}

	r.logger.Debug("creating platform FDS identifier mapping", "fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID, "platformTenantID", platformTenantID, "platformUserID", platformUserID)

	// ON CONFLICT (fds_tenant_id, fds_user_id) DO NOTHING — silently skips duplicate mappings.
	if err := r.writerDB.
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "fds_tenant_id"}, {Name: "fds_user_id"}},
			DoNothing: true,
		}).
		Create(&dataItem).Error; err != nil {
		r.logger.Error("failed to create platform FDS identifier mapping", "fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID, "platformTenantID", platformTenantID, "platformUserID", platformUserID, "error", err)
		return err
	}

	return nil
}
