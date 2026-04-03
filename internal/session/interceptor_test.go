package session_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/sample-go/item-service/internal/session"
)

var (
	testTenantID = uuid.MustParse("10000000-0000-0000-0000-000000000001")
	testUserID   = uuid.MustParse("20000000-0000-0000-0000-000000000002")

	testFDSIssuer = "https://fds.example.com"
)

// noopHandler is a gRPC handler that returns the RequestSession from the context.
func noopHandler(ctx context.Context, _ any) (any, error) {
	sess := session.FromContext(ctx)
	return sess, nil
}

func invokeInterceptor(md metadata.MD, fdsIssuer string) (*session.RequestSession, error) {
	ctx := metadata.NewIncomingContext(context.Background(), md)
	interceptor := session.UnaryInterceptor(slog.Default(), fdsIssuer)

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	resp, err := interceptor(ctx, nil, info, noopHandler)
	if err != nil {
		return nil, err
	}
	return resp.(*session.RequestSession), nil
}

// makeTestJWT builds a minimal unsigned JWT with the given claims map.
func makeTestJWT(claims map[string]any) string {
	payload, _ := json.Marshal(claims)
	encodedHeader := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	return fmt.Sprintf("%s.%s.sig", encodedHeader, encodedPayload)
}

// ─── Header-based authentication ────────────────────────────────────────────

func TestUnaryInterceptor_Success_Headers(t *testing.T) {
	md := metadata.Pairs("tenant_id", testTenantID.String(), "user_id", testUserID.String())
	sess, err := invokeInterceptor(md, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.TenantID != testTenantID {
		t.Errorf("TenantID = %q, want %q", sess.TenantID, testTenantID)
	}
	if sess.UserID != testUserID {
		t.Errorf("UserID = %q, want %q", sess.UserID, testUserID)
	}
}

func TestUnaryInterceptor_MissingMetadata(t *testing.T) {
	interceptor := session.UnaryInterceptor(slog.Default(), "")
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	_, err := interceptor(context.Background(), nil, info, noopHandler)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Errorf("code = %v, want %v", st.Code(), codes.Unauthenticated)
	}
}

func TestUnaryInterceptor_MissingTenantID(t *testing.T) {
	md := metadata.Pairs("user_id", testUserID.String())
	_, err := invokeInterceptor(md, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("code = %v, want %v", st.Code(), codes.Unauthenticated)
	}
}

func TestUnaryInterceptor_MissingUserID(t *testing.T) {
	md := metadata.Pairs("tenant_id", testTenantID.String())
	_, err := invokeInterceptor(md, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("code = %v, want %v", st.Code(), codes.Unauthenticated)
	}
}

func TestUnaryInterceptor_EmptyValues(t *testing.T) {
	md := metadata.Pairs("tenant_id", "", "user_id", "")
	_, err := invokeInterceptor(md, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("code = %v, want %v", st.Code(), codes.Unauthenticated)
	}
}

func TestUnaryInterceptor_InvalidTenantUUID(t *testing.T) {
	md := metadata.Pairs("tenant_id", "not-a-uuid", "user_id", testUserID.String())
	_, err := invokeInterceptor(md, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
}

func TestUnaryInterceptor_InvalidUserUUID(t *testing.T) {
	md := metadata.Pairs("tenant_id", testTenantID.String(), "user_id", "not-a-uuid")
	_, err := invokeInterceptor(md, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
}

// ─── JWT Bearer authentication ───────────────────────────────────────────────

func TestUnaryInterceptor_JWT_Success(t *testing.T) {
	token := makeTestJWT(map[string]any{
		"tenant_id": testTenantID.String(),
		"user_id":   testUserID.String(),
		"email":     "user@example.com",
	})
	md := metadata.Pairs("authorization", "Bearer "+token)
	sess, err := invokeInterceptor(md, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.TenantID != testTenantID {
		t.Errorf("TenantID = %q, want %q", sess.TenantID, testTenantID)
	}
	if sess.UserID != testUserID {
		t.Errorf("UserID = %q, want %q", sess.UserID, testUserID)
	}
	if sess.Email != "user@example.com" {
		t.Errorf("Email = %q, want %q", sess.Email, "user@example.com")
	}
	if sess.FDSClaims != nil {
		t.Errorf("expected no FDSClaims for non-FDS token, got %+v", sess.FDSClaims)
	}
}

func TestUnaryInterceptor_JWT_FDSIssuer_PopulatesFDSClaims(t *testing.T) {
	token := makeTestJWT(map[string]any{
		"iss":                  testFDSIssuer,
		"tenant_id":            testTenantID.String(),
		"user_id":              testUserID.String(),
		"sws.samauth.ten":      "fds-tenant-001",
		"sws.samauth.ten.user": "fds-user-001",
		"email":                "fdsuser@fds.example.com",
	})
	md := metadata.Pairs("authorization", "Bearer "+token)
	sess, err := invokeInterceptor(md, testFDSIssuer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.FDSClaims == nil {
		t.Fatal("expected FDSClaims to be populated for FDS token")
	}
	if sess.FDSClaims.TenantID != "fds-tenant-001" {
		t.Errorf("FDSClaims.TenantID = %q, want %q", sess.FDSClaims.TenantID, "fds-tenant-001")
	}
	if sess.FDSClaims.UserID != "fds-user-001" {
		t.Errorf("FDSClaims.UserID = %q, want %q", sess.FDSClaims.UserID, "fds-user-001")
	}
	if sess.FDSClaims.UserEmail != "fdsuser@fds.example.com" {
		t.Errorf("FDSClaims.UserEmail = %q, want %q", sess.FDSClaims.UserEmail, "fdsuser@fds.example.com")
	}
}

func TestUnaryInterceptor_JWT_FDSIssuer_NotMatchedWhenDifferent(t *testing.T) {
	token := makeTestJWT(map[string]any{
		"iss":       "https://other-issuer.example.com",
		"tenant_id": testTenantID.String(),
		"user_id":   testUserID.String(),
	})
	md := metadata.Pairs("authorization", "Bearer "+token)
	sess, err := invokeInterceptor(md, testFDSIssuer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.FDSClaims != nil {
		t.Errorf("expected no FDSClaims when issuer doesn't match, got %+v", sess.FDSClaims)
	}
}

func TestUnaryInterceptor_JWT_MalformedToken(t *testing.T) {
	md := metadata.Pairs("authorization", "Bearer not.a.valid")
	_, err := invokeInterceptor(md, "")
	if err == nil {
		t.Fatal("expected error for malformed JWT, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("code = %v, want %v", st.Code(), codes.Unauthenticated)
	}
}

func TestUnaryInterceptor_JWT_InvalidTenantUUIDInClaims(t *testing.T) {
	token := makeTestJWT(map[string]any{
		"tenant_id": "not-a-uuid",
		"user_id":   testUserID.String(),
	})
	md := metadata.Pairs("authorization", "Bearer "+token)
	_, err := invokeInterceptor(md, "")
	if err == nil {
		t.Fatal("expected error for invalid tenant UUID in JWT, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("code = %v, want %v", st.Code(), codes.Unauthenticated)
	}
}

// ─── Traces ───────────────────────────────────────────────────────────────────

func TestUnaryInterceptor_Traces_PopulatedFromHeaders(t *testing.T) {
	md := metadata.Pairs(
		"tenant_id", testTenantID.String(),
		"user_id", testUserID.String(),
		"x-b3-traceid", "abc123traceid",
		"x-b3-spanid", "span456",
		"x-b3-parentspanid", "parent789",
		"x-request-id", "req-001",
		"x-correlation-id", "corr-002",
	)
	sess, err := invokeInterceptor(md, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.Traces.TraceID != "abc123traceid" {
		t.Errorf("TraceID = %q, want %q", sess.Traces.TraceID, "abc123traceid")
	}
	if sess.Traces.SpanID != "span456" {
		t.Errorf("SpanID = %q, want %q", sess.Traces.SpanID, "span456")
	}
	if sess.Traces.ParentSpanID != "parent789" {
		t.Errorf("ParentSpanID = %q, want %q", sess.Traces.ParentSpanID, "parent789")
	}
	if sess.Traces.RequestID != "req-001" {
		t.Errorf("RequestID = %q, want %q", sess.Traces.RequestID, "req-001")
	}
	if sess.Traces.CorrelationID != "corr-002" {
		t.Errorf("CorrelationID = %q, want %q", sess.Traces.CorrelationID, "corr-002")
	}
}

func TestUnaryInterceptor_Traces_EmptyWhenAbsent(t *testing.T) {
	md := metadata.Pairs("tenant_id", testTenantID.String(), "user_id", testUserID.String())
	sess, err := invokeInterceptor(md, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.Traces.TraceID != "" || sess.Traces.SpanID != "" || sess.Traces.RequestID != "" {
		t.Errorf("expected empty traces, got %+v", sess.Traces)
	}
}

// ─── Context helpers ──────────────────────────────────────────────────────────

func TestFromContext_NilWhenNoSession(t *testing.T) {
	sess := session.FromContext(context.Background())
	if sess != nil {
		t.Errorf("expected nil session, got %+v", sess)
	}
}

func TestWithSession_RoundTrip(t *testing.T) {
	s := &session.RequestSession{TenantID: testTenantID, UserID: testUserID}
	ctx := session.WithSession(context.Background(), s)
	got := session.FromContext(ctx)
	if got == nil {
		t.Fatal("expected session, got nil")
	}
	if got.TenantID != testTenantID || got.UserID != testUserID {
		t.Errorf("session = %+v, want TenantID=%s UserID=%s", got, testTenantID, testUserID)
	}
}

// ─── Claims map ───────────────────────────────────────────────────────────────

func TestUnaryInterceptor_Headers_PopulatesClaimsMap(t *testing.T) {
	md := metadata.Pairs("tenant_id", testTenantID.String(), "user_id", testUserID.String())
	sess, err := invokeInterceptor(md, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.Claims == nil {
		t.Fatal("expected Claims to be populated from headers")
	}
	if sess.Claims["tenant_id"] != testTenantID.String() {
		t.Errorf("Claims[tenant_id] = %q, want %q", sess.Claims["tenant_id"], testTenantID.String())
	}
	if sess.Claims["user_id"] != testUserID.String() {
		t.Errorf("Claims[user_id] = %q, want %q", sess.Claims["user_id"], testUserID.String())
	}
}

func TestUnaryInterceptor_JWT_PopulatesClaimsMap(t *testing.T) {
	token := makeTestJWT(map[string]any{
		"tenant_id":    testTenantID.String(),
		"user_id":      testUserID.String(),
		"email":        "user@example.com",
		"culture_code": "en_US",
		"custom_field": "custom_value",
	})
	md := metadata.Pairs("authorization", "Bearer "+token)
	sess, err := invokeInterceptor(md, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.Claims == nil {
		t.Fatal("expected Claims to be populated from JWT")
	}
	if sess.Claims["email"] != "user@example.com" {
		t.Errorf("Claims[email] = %q, want %q", sess.Claims["email"], "user@example.com")
	}
	if sess.Claims["custom_field"] != "custom_value" {
		t.Errorf("Claims[custom_field] = %q, want %q", sess.Claims["custom_field"], "custom_value")
	}
}

// ─── JWT raw token stored ─────────────────────────────────────────────────────

func TestUnaryInterceptor_JWT_StoresRawToken(t *testing.T) {
	token := makeTestJWT(map[string]any{
		"tenant_id": testTenantID.String(),
		"user_id":   testUserID.String(),
	})
	md := metadata.Pairs("authorization", "Bearer "+token)
	sess, err := invokeInterceptor(md, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.JWT != token {
		t.Errorf("JWT = %q, want %q", sess.JWT, token)
	}
}

func TestUnaryInterceptor_Headers_NoJWTStored(t *testing.T) {
	md := metadata.Pairs("tenant_id", testTenantID.String(), "user_id", testUserID.String())
	sess, err := invokeInterceptor(md, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.JWT != "" {
		t.Errorf("expected empty JWT for header-based auth, got %q", sess.JWT)
	}
}

// ─── Timestamp ────────────────────────────────────────────────────────────────

func TestUnaryInterceptor_TimestampSet(t *testing.T) {
	md := metadata.Pairs("tenant_id", testTenantID.String(), "user_id", testUserID.String())
	sess, err := invokeInterceptor(md, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.Timestamp.IsZero() {
		t.Error("expected Timestamp to be set, got zero value")
	}
}

// ─── GetLocale ────────────────────────────────────────────────────────────────

func TestGetLocale_ParsesUnderscoreFormat(t *testing.T) {
	token := makeTestJWT(map[string]any{
		"tenant_id":    testTenantID.String(),
		"user_id":      testUserID.String(),
		"culture_code": "en_US",
	})
	md := metadata.Pairs("authorization", "Bearer "+token)
	sess, err := invokeInterceptor(md, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	locale := sess.GetLocale()
	if locale == nil {
		t.Fatal("expected Locale, got nil")
	}
	if locale.Language != "en" {
		t.Errorf("Language = %q, want %q", locale.Language, "en")
	}
	if locale.Region != "US" {
		t.Errorf("Region = %q, want %q", locale.Region, "US")
	}
}

func TestGetLocale_ParsesHyphenFormat(t *testing.T) {
	s := &session.RequestSession{CultureCode: "fr-FR"}
	locale := s.GetLocale()
	if locale == nil {
		t.Fatal("expected Locale, got nil")
	}
	if locale.Language != "fr" || locale.Region != "FR" {
		t.Errorf("Locale = {%s %s}, want {fr FR}", locale.Language, locale.Region)
	}
}

func TestGetLocale_NilWhenBlank(t *testing.T) {
	s := &session.RequestSession{}
	if locale := s.GetLocale(); locale != nil {
		t.Errorf("expected nil Locale for blank CultureCode, got %+v", locale)
	}
}

func TestGetLocale_NilWhenInvalid(t *testing.T) {
	s := &session.RequestSession{CultureCode: "invalid"}
	if locale := s.GetLocale(); locale != nil {
		t.Errorf("expected nil Locale for single-segment culture code, got %+v", locale)
	}
}

func TestGetLocale_CachesResult(t *testing.T) {
	s := &session.RequestSession{CultureCode: "de_DE"}
	first := s.GetLocale()
	second := s.GetLocale()
	if first != second {
		t.Error("expected GetLocale to return the same cached pointer on repeated calls")
	}
}

// ─── SetCustomClaim ───────────────────────────────────────────────────────────

func TestSetCustomClaim_AddsNewKey(t *testing.T) {
	s := &session.RequestSession{}
	s.SetCustomClaim("role", "admin")
	if s.Claims["role"] != "admin" {
		t.Errorf("Claims[role] = %q, want %q", s.Claims["role"], "admin")
	}
}

func TestSetCustomClaim_DoesNotOverwriteExisting(t *testing.T) {
	s := &session.RequestSession{Claims: map[string]string{"role": "viewer"}}
	s.SetCustomClaim("role", "admin")
	if s.Claims["role"] != "viewer" {
		t.Errorf("Claims[role] = %q, want %q (should not overwrite)", s.Claims["role"], "viewer")
	}
}

func TestSetCustomClaim_InitialisesNilMap(t *testing.T) {
	s := &session.RequestSession{}
	s.SetCustomClaim("key", "value")
	if s.Claims == nil {
		t.Error("expected Claims map to be initialised, got nil")
	}
}
