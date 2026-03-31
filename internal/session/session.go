package session

import "context"

// RequestSession holds per-request identity and context extracted from gRPC metadata headers.
// Extend this struct to include additional claims (e.g. roles, email) when JWT parsing is added.
type RequestSession struct {
	TenantID string
	UserID   string
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
