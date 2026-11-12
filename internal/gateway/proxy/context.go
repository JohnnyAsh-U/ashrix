package proxy

import (
	"context"
)

// ============================================================
// retrieve Session from request context.
// ============================================================

type contextKey string

var (
	ConnectorID              contextKey = "ConnectorID"
	AppID                    contextKey = "AppID"
)

func ConnectorIDFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(ConnectorID).(string)
	return id
}

func AppIDFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(AppID).(string)
	return id
}


