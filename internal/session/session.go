package session

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Traces holds distributed tracing identifiers extracted from incoming request metadata.
// Populated from standard B3 / OpenTelemetry propagation headers.
type Traces struct {
	TraceID       string // x-b3-traceid
	SpanID        string // x-b3-spanid
	ParentSpanID  string // x-b3-parentspanid
	RequestID     string // x-request-id
	CorrelationID string // x-correlation-id
}

// FDSClaims holds the raw identity claims extracted from a Federated Data System (FDS)
// issued JWT token. These are the FDS-native identifiers that must be resolved to
// platform-level TenantID / UserID via an FDS identity resolution service.
// Non-nil only when the incoming JWT was issued by the configured FDS issuer.
type FDSClaims struct {
	TenantID  string // fds_tenant_id claim
	UserID    string // fds_user_id claim
	UserEmail string // email claim
}

// Locale represents a parsed culture code (e.g. "en_US" → Language="en", Region="US").
// Derived lazily from CultureCode via GetLocale().
type Locale struct {
	Language string // BCP 47 language subtag (e.g. "en")
	Region   string // ISO 3166-1 region subtag (e.g. "US")
}

// RequestSession holds per-request identity, raw claims, traces, and context extracted
// from gRPC metadata headers. Retrieve it in any handler via session.FromContext(ctx).
type RequestSession struct {
	TenantID    uuid.UUID
	UserID      uuid.UUID
	Email       string
	CultureCode string

	// Claims holds all raw key/value pairs from the JWT payload plus any custom claims
	// added via SetCustomClaim. This mirrors the Java RequestSession.claims map and allows
	// downstream code to access non-standard JWT claims without modifying this struct.
	Claims map[string]string

	// JWT is the raw Bearer token string extracted from the Authorization header.
	JWT string

	// Timestamp is set to the UTC time at which the session was initialised by the interceptor.
	Timestamp time.Time

	Traces    Traces
	FDSClaims *FDSClaims // non-nil when the Bearer token was issued by the FDS issuer

	localeMu sync.Mutex
	locale   *Locale
}

// GetLocale parses CultureCode (e.g. "en_US" or "en-US") into a Locale.
// The result is cached after the first call. Returns nil if CultureCode is blank
// or cannot be parsed into a language/region pair.
func (s *RequestSession) GetLocale() *Locale {
	s.localeMu.Lock()
	defer s.localeMu.Unlock()

	if s.locale != nil {
		return s.locale
	}
	if s.CultureCode == "" {
		return nil
	}

	// Support both "en_US" (Java convention) and "en-US" (BCP 47).
	parts := strings.FieldsFunc(s.CultureCode, func(r rune) bool { return r == '_' || r == '-' })
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return nil
	}
	s.locale = &Locale{Language: parts[0], Region: parts[1]}
	return s.locale
}

// SetCustomClaim adds a claim to the Claims map only when the key is not already present,
// mirroring Java's RequestSession.setCustomClaims (putIfAbsent behaviour).
// The Claims map is initialised lazily if nil.
func (s *RequestSession) SetCustomClaim(key, value string) {
	if s.Claims == nil {
		s.Claims = make(map[string]string)
	}
	if _, exists := s.Claims[key]; !exists {
		s.Claims[key] = value
	}
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
