package session

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// Identity headers (fallback when no Bearer token is present).
	HeaderTenantID      = "tenant_id"
	HeaderUserID        = "user_id"
	HeaderAuthorization = "authorization"

	// JWT standard claim keys.
	ClaimTenantID    = "tenant_id"
	ClaimUserID      = "user_id"
	ClaimEmail       = "email"
	ClaimCultureCode = "culture_code"

	// FDS-specific JWT claim keys (present when iss == fdsIssuer).
	FDSClaimTenantID  = "sws.samauth.ten"
	FDSClaimUserID    = "sws.samauth.ten.user"
	FDSClaimUserEmail = "email"
	FDSClaimIssuer    = "iss"

	// Distributed tracing headers (B3 / OpenTelemetry).
	HeaderTraceID       = "x-b3-traceid"
	HeaderSpanID        = "x-b3-spanid"
	HeaderParentSpanID  = "x-b3-parentspanid"
	HeaderRequestID     = "x-request-id"
	HeaderCorrelationID = "x-correlation-id"
)

// sessionHeaders lists all HTTP headers that the REST gateway should forward
// directly into gRPC metadata without the default "grpcgateway-" prefix.
var sessionHeaders = map[string]bool{
	HeaderTenantID:      true,
	HeaderUserID:        true,
	HeaderAuthorization: true,
	HeaderTraceID:       true,
	HeaderSpanID:        true,
	HeaderParentSpanID:  true,
	HeaderRequestID:     true,
	HeaderCorrelationID: true,
}

// GatewayHeaderMatcher forwards session-related HTTP headers directly into
// gRPC metadata without the "grpcgateway-" prefix. All other headers fall
// through to the default matcher.
func GatewayHeaderMatcher(key string) (string, bool) {
	if sessionHeaders[strings.ToLower(key)] {
		return strings.ToLower(key), true
	}
	return runtime.DefaultHeaderMatcher(key)
}

// UnaryInterceptor returns a gRPC unary server interceptor that:
//   - Extracts distributed tracing headers into RequestSession.Traces (always)
//   - Authenticates via JWT Bearer token (Authorization header): decodes the payload,
//     populates TenantID / UserID / Email / CultureCode, and detects FDS-issued tokens
//     by comparing the "iss" claim against fdsIssuer — capturing raw FDS identifiers
//     in RequestSession.FDSClaims for downstream platform identity resolution
//   - Falls back to explicit tenant_id / user_id headers when no Bearer token is present
//
// NOTE: JWT signature verification is intentionally omitted here. Add JWKS-based
// signature validation against the issuer's public keys before deploying to production.
func UnaryInterceptor(logger *slog.Logger, fdsIssuer string) grpc.UnaryServerInterceptor {
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

		sess := &RequestSession{
			Traces:    extractTraces(md),
			Timestamp: time.Now().UTC(),
		}

		authHeader := firstValue(md, HeaderAuthorization)
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			token := authHeader[7:] // trim "bearer " (7 chars, case-insensitive safe)
			sess.JWT = token
			if err := populateSessionFromJWT(ctx, sess, token, fdsIssuer, logger, info.FullMethod); err != nil {
				logger.WarnContext(ctx, "JWT authentication failed",
					"method", info.FullMethod, "error", err)
				return nil, status.Error(codes.Unauthenticated, err.Error())
			}
		} else {
			if err := populateSessionFromHeaders(sess, md, logger, info.FullMethod); err != nil {
				logger.WarnContext(ctx, "header authentication failed",
					"method", info.FullMethod, "error", err)
				return nil, err
			}
		}

		logger.DebugContext(ctx, "request session initialised",
			"method", info.FullMethod,
			"tenantId", sess.TenantID,
			"userId", sess.UserID,
			"hasFDSClaims", sess.FDSClaims != nil,
			"traceId", sess.Traces.TraceID,
			"requestId", sess.Traces.RequestID,
		)

		ctx = WithSession(ctx, sess)
		return handler(ctx, req)
	}
}

// populateSessionFromJWT decodes the JWT payload and extracts identity claims.
// All JWT payload entries are stored in sess.Claims as string values.
// When the "iss" claim matches fdsIssuer the raw FDS identifiers are captured in
// sess.FDSClaims for downstream FDS-to-platform identity resolution.
func populateSessionFromJWT(_ context.Context, sess *RequestSession, token, fdsIssuer string, logger *slog.Logger, method string) error {
	claims, err := decodeJWTPayload(token)
	if err != nil {
		return fmt.Errorf("invalid JWT: %w", err)
	}

	// Store all JWT claims as strings (mirrors Java session.setClaims).
	claimsStr := make(map[string]string, len(claims))
	for k, v := range claims {
		claimsStr[k] = fmt.Sprintf("%v", v)
	}
	sess.Claims = claimsStr

	// Detect FDS-issued token and capture FDS-native identifiers.
	// Downstream handlers that need platform UUIDs should check sess.FDSClaims != nil
	// and call PlatformFDSIdentifierMapService.GetPlatformDetailsbyFDSIdentifiers.
	if fdsIssuer != "" && stringClaim(claims, FDSClaimIssuer) == fdsIssuer {
		sess.FDSClaims = &FDSClaims{
			TenantID:  stringClaim(claims, FDSClaimTenantID),
			UserID:    stringClaim(claims, FDSClaimUserID),
			UserEmail: stringClaim(claims, FDSClaimUserEmail),
		}
		logger.Debug("FDS token detected — platform identity resolution required",
			"method", method,
			"fdsTenantId", sess.FDSClaims.TenantID,
			"fdsUserId", sess.FDSClaims.UserID,
		)
	}

	// Populate TenantID / UserID from standard JWT claims.
	tenantIDStr := stringClaim(claims, ClaimTenantID)
	userIDStr := stringClaim(claims, ClaimUserID)

	if tenantIDStr != "" {
		parsed, err := uuid.Parse(tenantIDStr)
		if err != nil {
			return fmt.Errorf("invalid tenant_id claim in JWT: %w", err)
		}
		sess.TenantID = parsed
	}
	if userIDStr != "" {
		parsed, err := uuid.Parse(userIDStr)
		if err != nil {
			return fmt.Errorf("invalid user_id claim in JWT: %w", err)
		}
		sess.UserID = parsed
	}

	sess.Email = stringClaim(claims, ClaimEmail)
	sess.CultureCode = stringClaim(claims, ClaimCultureCode)
	return nil
}

// populateSessionFromHeaders falls back to explicit tenant_id / user_id metadata headers.
// Sets sess.Claims with the two identity keys so downstream code can use Claims uniformly.
func populateSessionFromHeaders(sess *RequestSession, md metadata.MD, logger *slog.Logger, method string) error {
	tenantID := firstValue(md, HeaderTenantID)
	userID := firstValue(md, HeaderUserID)

	if tenantID == "" || userID == "" {
		logger.Warn("missing required session headers",
			"method", method,
			"tenant_id", tenantID,
			"user_id", userID,
		)
		return status.Error(codes.Unauthenticated, "authorization token or tenant_id and user_id headers are required")
	}

	parsedTenantID, err := uuid.Parse(tenantID)
	if err != nil {
		return status.Error(codes.InvalidArgument, "tenant_id must be a valid UUID")
	}
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return status.Error(codes.InvalidArgument, "user_id must be a valid UUID")
	}

	sess.TenantID = parsedTenantID
	sess.UserID = parsedUserID
	sess.Claims = map[string]string{
		ClaimTenantID: tenantID,
		ClaimUserID:   userID,
	}
	return nil
}

// extractTraces reads distributed tracing headers from incoming metadata.
func extractTraces(md metadata.MD) Traces {
	return Traces{
		TraceID:       firstValue(md, HeaderTraceID),
		SpanID:        firstValue(md, HeaderSpanID),
		ParentSpanID:  firstValue(md, HeaderParentSpanID),
		RequestID:     firstValue(md, HeaderRequestID),
		CorrelationID: firstValue(md, HeaderCorrelationID),
	}
}

// decodeJWTPayload base64url-decodes the JWT payload segment and unmarshals it.
// NOTE: This does NOT verify the JWT signature. Add JWKS-based signature validation
// against the issuer's public keys before deploying to production.
func decodeJWTPayload(token string) (map[string]any, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed JWT: expected 3 segments, got %d", len(parts))
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWT payload: %w", err)
	}
	return claims, nil
}

// stringClaim safely extracts a string value from a JWT claims map.
func stringClaim(claims map[string]any, key string) string {
	if v, ok := claims[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// firstValue returns the first value for the given metadata key, or "".
func firstValue(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}
