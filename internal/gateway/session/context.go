package session

import (
	"context"

	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"

)

// ============================================================
// retrieve Session from request context.
// ============================================================

type contextKey string

var (
	ConnectorID contextKey = "ConnectorID"
	AppID contextKey = "AppID"
	AppIsPublic contextKey = "AppIsPublic"
	Identity contextKey = "Identity"
	SessionID contextKey = "SessionID"
)


func ConnectorIDFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(ConnectorID).(string)
	return id
}

func AppIDFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(AppID).(string)
	return id
}

func AppIsPublicFromCtx(ctx context.Context) bool {
	isPublic, _ := ctx.Value(AppIsPublic).(bool)
	return isPublic
}

func IdentityFromCtx(ctx context.Context) *gen.NormalizedIdentity {
	id, _ := ctx.Value(Identity).(*gen.NormalizedIdentity)
	return id
}

func SessionIDFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(SessionID).(string)
	return id
}
