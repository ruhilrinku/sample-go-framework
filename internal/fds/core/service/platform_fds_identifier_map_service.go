package service

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/sample-go/item-service/internal/fds/core/port"
)

type PlatformFDSIdentifierMapService struct {
	repo   port.PlatformFdsIdentifierMapRepository
	logger *slog.Logger
}

func NewPlatformFDSIdentifierMapService(repo port.PlatformFdsIdentifierMapRepository, logger *slog.Logger) *PlatformFDSIdentifierMapService {
	return &PlatformFDSIdentifierMapService{
		repo:   repo,
		logger: logger.With("component", "platform-fds-identifier-map-service"),
	}
}

func (s *PlatformFDSIdentifierMapService) GetPlatformDetailsbyFDSIdentifiers(ctx context.Context, fdsTenantID, fdsUserID string) (uuid.UUID, uuid.UUID, error) {
	s.logger.InfoContext(ctx, "fetching platform details for FDS tenant and user", "fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID)

	platformTenantID, platformUserID, err := s.repo.GetPlatformDetailsbyFDSIdentifiers(ctx, fdsTenantID, fdsUserID)

	if err != nil {
		s.logger.ErrorContext(ctx, "failed to fetch platform details", "fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID, "error", err)
		return uuid.Nil, uuid.Nil, err
	}

	s.logger.InfoContext(ctx, "fetched platform details", "fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID, "platformTenantID", platformTenantID, "platformUserID", platformUserID)
	return platformTenantID, platformUserID, nil
}

func (s *PlatformFDSIdentifierMapService) CreatePlatformFdsIdentifierMapping(ctx context.Context, fdsTenantID, fdsUserID string, platformTenantID, platformUserID uuid.UUID) error {
	s.logger.InfoContext(ctx, "creating platform-FDS identifier mapping", "fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID, "platformTenantID", platformTenantID, "platformUserID", platformUserID)

	if err := s.repo.CreatePlatformFdsIdentifierMapping(ctx, fdsTenantID, fdsUserID, platformTenantID, platformUserID); err != nil {
		s.logger.ErrorContext(ctx, "failed to create platform-FDS identifier mapping", "fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID, "platformTenantID", platformTenantID, "platformUserID", platformUserID, "error", err)
		return err
	}

	s.logger.InfoContext(ctx, "created platform-FDS identifier mapping", "fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID, "platformTenantID", platformTenantID, "platformUserID", platformUserID)
	return nil
}
