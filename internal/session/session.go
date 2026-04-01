package session

import (
	"context"

	"github.com/google/uuid"
)

// RequestSession holds per-request identity and context extracted from gRPC metadata headers.
// Extend this struct to include additional claims (e.g. roles, email) when JWT parsing is added.
type RequestSession struct {
	TenantID uuid.UUID
	UserID   uuid.UUID
}

type contextKey struct{}

// WithSession returns a new context carrying the given RequestSession.
func WithSession(ctx context.Context, s *RequestSession) context.Context {
	return context.WithValue(ctx, contextKey{}, s)
}

// FromContext retrieves the RequestSession from the context, or nil if not present.
func FromContext(ctx context.Context) *RequestSession {
	s, _ := ctx.Value(contextKey{}).(*RequestSession)
	return s
}
