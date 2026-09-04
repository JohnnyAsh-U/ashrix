package logging

import "context"

type accessContextKey struct{}

type AccessContext struct {
	AppID       string
	AppName     string
	ConnectorID string

	UserID    string
	UserEmail string
	TenantID  string

	PolicyID   string
	Decision   string
	DenyReason string
}

func WithAccessContext(ctx context.Context) (context.Context, *AccessContext) {
	ac := &AccessContext{}

	return context.WithValue(ctx, accessContextKey{}, ac), ac
}

func AccessContextFromContext(ctx context.Context) *AccessContext {
	ac, _ := ctx.Value(accessContextKey{}).(*AccessContext)
	return ac
}