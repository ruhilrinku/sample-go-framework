package session_test

import (
	"context"
	"log/slog"
	"testing"

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

func TestUnaryInterceptor_Success(t *testing.T) {
	md := metadata.Pairs("x-tenant-id", "tenant-abc", "x-user-id", "user-123")
	sess, err := invokeInterceptor(md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.TenantID != "tenant-abc" {
		t.Errorf("TenantID = %q, want %q", sess.TenantID, "tenant-abc")
	}
	if sess.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", sess.UserID, "user-123")
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
	md := metadata.Pairs("x-user-id", "user-123")
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
	md := metadata.Pairs("x-tenant-id", "tenant-abc")
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

func TestFromContext_NilWhenNoSession(t *testing.T) {
	sess := session.FromContext(context.Background())
	if sess != nil {
		t.Errorf("expected nil session, got %+v", sess)
	}
}

func TestWithSession_RoundTrip(t *testing.T) {
	s := &session.RequestSession{TenantID: "t1", UserID: "u1"}
	ctx := session.WithSession(context.Background(), s)
	got := session.FromContext(ctx)
	if got == nil {
		t.Fatal("expected session, got nil")
	}
	if got.TenantID != "t1" || got.UserID != "u1" {
		t.Errorf("session = %+v, want TenantID=t1 UserID=u1", got)
	}
}
