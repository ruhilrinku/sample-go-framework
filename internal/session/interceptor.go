package session

import (
	"context"
	"log/slog"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// Header keys expected in gRPC metadata (lowercase per gRPC convention).
	HeaderTenantID = "x-tenant-id"
	HeaderUserID   = "x-user-id"
)

// sessionHeaders lists the HTTP headers that should be forwarded as-is
// (without the grpcgateway- prefix) from the REST gateway to gRPC metadata.
var sessionHeaders = map[string]bool{
	HeaderTenantID: true,
	HeaderUserID:   true,
}

// GatewayHeaderMatcher is a grpc-gateway incoming header matcher that
// forwards session-related HTTP headers directly into gRPC metadata
// without the default "grpcgateway-" prefix. All other headers fall
// through to the default matcher.
func GatewayHeaderMatcher(key string) (string, bool) {
	if sessionHeaders[strings.ToLower(key)] {
		return strings.ToLower(key), true
	}
	return runtime.DefaultHeaderMatcher(key)
}

// UnaryInterceptor returns a gRPC unary server interceptor that extracts
// identity headers from incoming metadata and initialises a RequestSession
// on the context. All downstream handlers and services can retrieve the
// session via session.FromContext(ctx).
//
// Currently reads x-tenant-id and x-user-id from metadata.
// Extend this function to parse a JWT bearer token and populate additional
// RequestSession fields when needed.
func UnaryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			logger.WarnContext(ctx, "no metadata in request", "method", info.FullMethod)
			return nil, status.Error(codes.Unauthenticated, "missing request metadata")
		}

		tenantID := firstValue(md, HeaderTenantID)
		userID := firstValue(md, HeaderUserID)

		if tenantID == "" || userID == "" {
			logger.WarnContext(ctx, "missing required session headers",
				"method", info.FullMethod,
				"x-tenant-id", tenantID,
				"x-user-id", userID,
			)
			return nil, status.Error(codes.Unauthenticated, "x-tenant-id and x-user-id headers are required")
		}

		sess := &RequestSession{
			TenantID: tenantID,
			UserID:   userID,
		}

		logger.DebugContext(ctx, "request session initialised",
			"method", info.FullMethod,
			"tenantId", sess.TenantID,
			"userId", sess.UserID,
		)

		ctx = WithSession(ctx, sess)
		return handler(ctx, req)
	}
}

// firstValue returns the first value for the given metadata key, or "".
func firstValue(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}
