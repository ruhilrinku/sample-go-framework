package grpc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	fdsv1 "github.com/sample-go/item-service/gen/pb/fds/v1"
)

// FDSGRPCClient is a driven adapter that implements port.PlatformFdsIdentifierMapRepository
// by calling the external FDS gRPC service.
type FDSGRPCClient struct {
	client fdsv1.FdsServiceClient
	logger *slog.Logger
}

// NewFDSGRPCClient dials the FDS gRPC service and returns a ready-to-use client adapter.
// The caller is responsible for closing the returned *grpc.ClientConn when done.
func NewFDSGRPCClient(target string, logger *slog.Logger) (*FDSGRPCClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("dial FDS gRPC service at %s: %w", target, err)
	}
	return &FDSGRPCClient{
		client: fdsv1.NewFdsServiceClient(conn),
		logger: logger.With("component", "fds-grpc-client"),
	}, conn, nil
}

// GetPlatformDetailsbyFDSIdentifiers calls FdsService.GetPlatformIdentifiers and
// returns the resolved platform tenant UUID and user UUID.
func (c *FDSGRPCClient) GetPlatformDetailsbyFDSIdentifiers(ctx context.Context, fdsTenantID, fdsUserID string) (uuid.UUID, uuid.UUID, error) {
	c.logger.DebugContext(ctx, "calling FDS service for platform identifiers",
		"fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID)

	resp, err := c.client.GetPlatformIdentifiers(ctx, &fdsv1.FdsIdentifiers{
		FdsTenantId: fdsTenantID,
		FdsUserId:   fdsUserID,
	})
	if err != nil {
		c.logger.ErrorContext(ctx, "FDS service call failed",
			"fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID, "error", err)
		return uuid.Nil, uuid.Nil, fmt.Errorf("FDS GetPlatformIdentifiers: %w", err)
	}

	platformTenantID, err := uuid.Parse(resp.PlatformTenantId)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("FDS returned invalid platform_tenant_id %q: %w", resp.PlatformTenantId, err)
	}
	platformUserID, err := uuid.Parse(resp.PlatformUserId)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("FDS returned invalid platform_user_id %q: %w", resp.PlatformUserId, err)
	}

	c.logger.DebugContext(ctx, "FDS service resolved platform identifiers",
		"fdsTenantID", fdsTenantID, "fdsUserID", fdsUserID,
		"platformTenantID", platformTenantID, "platformUserID", platformUserID)

	return platformTenantID, platformUserID, nil
}

// CreatePlatformFdsIdentifierMapping is a no-op for the gRPC adapter — mappings are
// owned by the FDS service and written via the postgres adapter.
func (c *FDSGRPCClient) CreatePlatformFdsIdentifierMapping(_ context.Context, _, _ string, _, _ uuid.UUID) error {
	return nil
}
