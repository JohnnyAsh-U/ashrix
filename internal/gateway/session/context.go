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
	Identitiy contextKey = "Identity"
)

var postureContextKey = ""


func ConnectorIDFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(ConnectorID).(string)
	return id
}

func AppIDFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(AppID).(string)
	return id
}

func IdentityFromCtx(ctx context.Context) *gen.NormalizedIdentity {
	id, _ := ctx.Value(Identitiy).(*gen.NormalizedIdentity)
	return id
}
