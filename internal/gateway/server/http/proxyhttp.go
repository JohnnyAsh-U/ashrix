package http_proxy

import (
	"net/http"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/session"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/google/uuid"
)


func (h *Handler) proxyHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	AppID := session.AppIDFromCtx(ctx)
	Identity := session.IdentityFromCtx(ctx)
	sessionID := session.SessionIDFromCtx(ctx)

	var userID, userEmail, userSessionID string
	if Identity != nil {
		userID = Identity.UserId
		userEmail = Identity.Email
	}

	if sessionID == "" {
		userSessionID = "no-session"
	}


	// Write request envelope + body to stream
	envelope := proto.RequestHeader{
		Method:     r.Method,
		AppId:      AppID,
		Path:       r.URL.Path,
		Query:      r.URL.RawQuery,
		UserId:     userID,    // Defaults to "" if Identity is nil
		UserEmail:  userEmail, // Defaults to "" if Identity is nil
		Headers:    flattenHeaders(r.Header),
		RequestId:  uuid.NewString(),
		SessionId:  userSessionID, // Defaults to "no-session" if sessionID is empty
		BodyLength: r.ContentLength,
	}

}