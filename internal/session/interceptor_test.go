package session_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/sample-go/item-service/internal/session"
)

// noopHandler is a gRPC handler that returns the RequestSession from the context.
func noopHandler(ctx context.Context, _ any) (any, error) {
	sess := session.FromContext(ctx)
	return sess, nil
}

func invokeInterceptor(md metadata.MD) (*session.RequestSession, error) {
	ctx := metadata.NewIncomingContext(context.Background(), md)
	interceptor := session.UnaryInterceptor(slog.Default())

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	resp, err := interceptor(ctx, nil, info, noopHandler)
	if err != nil {
		return nil, err
	}
	return resp.(*session.RequestSession), nil
}

var (
	testTenantID = uuid.MustParse("10000000-0000-0000-0000-000000000001")
	testUserID   = uuid.MustParse("20000000-0000-0000-0000-000000000002")
)

func TestUnaryInterceptor_Success(t *testing.T) {
	md := metadata.Pairs("x-tenant-id", testTenantID.String(), "x-user-id", testUserID.String())
	sess, err := invokeInterceptor(md)
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
	// No metadata at all
	interceptor := session.UnaryInterceptor(slog.Default())
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
	md := metadata.Pairs("x-user-id", testUserID.String())
	_, err := invokeInterceptor(md)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("code = %v, want %v", st.Code(), codes.Unauthenticated)
	}
}

func TestUnaryInterceptor_MissingUserID(t *testing.T) {
	md := metadata.Pairs("x-tenant-id", testTenantID.String())
	_, err := invokeInterceptor(md)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("code = %v, want %v", st.Code(), codes.Unauthenticated)
	}
}

func TestUnaryInterceptor_EmptyValues(t *testing.T) {
	md := metadata.Pairs("x-tenant-id", "", "x-user-id", "")
	_, err := invokeInterceptor(md)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("code = %v, want %v", st.Code(), codes.Unauthenticated)
	}
}

func TestUnaryInterceptor_InvalidTenantUUID(t *testing.T) {
	md := metadata.Pairs("x-tenant-id", "not-a-uuid", "x-user-id", testUserID.String())
	_, err := invokeInterceptor(md)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
}

func TestUnaryInterceptor_InvalidUserUUID(t *testing.T) {
	md := metadata.Pairs("x-tenant-id", testTenantID.String(), "x-user-id", "not-a-uuid")
	_, err := invokeInterceptor(md)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
}

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
