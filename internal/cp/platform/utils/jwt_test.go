package utils

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOTPTokenPurpose(t *testing.T) {
	jwtUtils := NewJwtUtils("access-secret", "refresh-secret", time.Minute, time.Hour)
	adminID := uuid.New()

	token, err := jwtUtils.IssueOTPToken(adminID, OTPPurposeEnroll, time.Now().Add(time.Minute))
	require.NoError(t, err)
	claims, err := jwtUtils.ParseOTPToken(token)
	require.NoError(t, err)
	require.Equal(t, adminID.String(), claims.AdminID)
	require.Equal(t, OTPPurposeEnroll, claims.Purpose)

	_, err = jwtUtils.IssueOTPToken(adminID, "access", time.Now().Add(time.Minute))
	require.Error(t, err)
}
