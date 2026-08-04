package posture

import "context"

// ============================================================
// 2. CONTEXT STORAGE (internal/posture/context.go)
// ============================================================
// Store/retrieve DevicePosture from request context.
// This is how the policy middleware accesses posture data.
// ============================================================

type contextKey struct{}

var postureContextKey = &contextKey{}

// WithContext stores a DevicePosture in the request context.
func WithContext(ctx context.Context, dp *DevicePosture) context.Context {
	return context.WithValue(ctx, postureContextKey, dp)
}

// FromContext retrieves a DevicePosture from the request context.
// Returns nil if no posture was collected.
func FromContext(ctx context.Context) (*DevicePosture, bool) {
	dp, ok := ctx.Value(postureContextKey).(*DevicePosture)
	return dp, ok
}