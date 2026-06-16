package auth

import (
	"context"
	"fmt"
	"time"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/google/uuid"
)

// Repository is the interface the service depends on.
type Repository interface {
	// Admins
	CreateAdmin(ctx context.Context, params store.CreateAdminParams) (store.Admin, error)
	GetAdminByEmail(ctx context.Context, email string) (store.Admin, error)
	GetAdminByEmailWithOrg(ctx context.Context, params store.GetAdminByEmailWithOrgParams) (store.Admin, error)
	GetAdminByID(ctx context.Context, id uuid.UUID) (store.Admin, error)
	UpdateAdminOTP(ctx context.Context, params store.UpdateAdminOTPParams) (store.Admin, error)
	ActivateAdmin(ctx context.Context, params store.ActivateAdminParams) (store.Admin, error)
	UpdateAdminPassword(ctx context.Context, params store.UpdateAdminPasswordParams) error

	// Setup tokens
	CreateSetupToken(ctx context.Context, params store.CreateSetupTokenParams) (store.AdminSetupToken, error)
	GetSetupToken(ctx context.Context, tokenHash string) (store.AdminSetupToken, error)
	MarkSetupTokenUsed(ctx context.Context, id uuid.UUID) error

	// Sessions
	CreateSession(ctx context.Context, params store.CreateSessionParams) (store.AdminSession, error)
	GetSession(ctx context.Context, refreshToken string) (store.AdminSession, error)
	RevokeSession(ctx context.Context, id uuid.UUID) error
	RevokeAllAdminSessions(ctx context.Context, adminID uuid.UUID) error

	// Password reset
	CreatePasswordResetToken(ctx context.Context, params store.CreatePasswordResetTokenParams) (store.PasswordResetToken, error)
	GetPasswordResetToken(ctx context.Context, tokenHash string) (store.PasswordResetToken, error)
	MarkPasswordResetTokenUsed(ctx context.Context, id uuid.UUID) error
}

type postgresRepository struct {
	q *store.Queries
}

func NewPostgresRepository(q *store.Queries) Repository {
	return &postgresRepository{q: q}
}

func (r *postgresRepository) CreateAdmin(ctx context.Context, params store.CreateAdminParams) (store.Admin, error) {
	return r.q.CreateAdmin(ctx, params)
}

func (r *postgresRepository) GetAdminByEmailWithOrg(ctx context.Context, params store.GetAdminByEmailWithOrgParams) (store.Admin, error) {
	return r.q.GetAdminByEmailWithOrg(ctx, params)
}

func (r *postgresRepository) GetAdminByEmail(ctx context.Context, email string) (store.Admin, error) {
	return r.q.GetAdminByEmail(ctx, email)
}

func (r *postgresRepository) GetAdminByID(ctx context.Context, id uuid.UUID) (store.Admin, error) {
	return r.q.GetAdminByID(ctx, id)
}

func (r *postgresRepository) UpdateAdminOTP(ctx context.Context, params store.UpdateAdminOTPParams) (store.Admin, error) {
	return r.q.UpdateAdminOTP(ctx, params)
}

func (r *postgresRepository) ActivateAdmin(ctx context.Context, params store.ActivateAdminParams) (store.Admin, error) {
	return r.q.ActivateAdmin(ctx, params)
}

func (r *postgresRepository) UpdateAdminPassword(ctx context.Context, params store.UpdateAdminPasswordParams) error {
	return r.q.UpdateAdminPassword(ctx, params)
}

func (r *postgresRepository) CreateSetupToken(ctx context.Context, params store.CreateSetupTokenParams) (store.AdminSetupToken, error) {
	row, err := r.q.CreateSetupToken(ctx, params)
	if err != nil {
		return store.AdminSetupToken{}, fmt.Errorf("auth.CreateSetupToken: %w", err)
	}
	return row, nil
}

func (r *postgresRepository) GetSetupToken(ctx context.Context, tokenHash string) (store.AdminSetupToken, error) {
	return r.q.GetSetupToken(ctx, tokenHash)
}

func (r *postgresRepository) MarkSetupTokenUsed(ctx context.Context, id uuid.UUID) error {
	return r.q.MarkSetupTokenUsed(ctx, id)
}

func (r *postgresRepository) CreateSession(ctx context.Context, params store.CreateSessionParams) (store.AdminSession, error) {
	row, err := r.q.CreateSession(ctx, params)
	if err != nil {
		return store.AdminSession{}, fmt.Errorf("auth.CreateSession: %w", err)
	}
	return row, nil
}

func (r *postgresRepository) GetSession(ctx context.Context, refreshToken string) (store.AdminSession, error) {
	return r.q.GetAdminSession(ctx, refreshToken)
}

func (r *postgresRepository) RevokeSession(ctx context.Context, id uuid.UUID) error {
	return r.q.RevokeAdminSession(ctx, id)
}

func (r *postgresRepository) RevokeAllAdminSessions(ctx context.Context, adminID uuid.UUID) error {
	return r.q.RevokeAllAdminSessionsForAdmin(ctx, adminID)
}

func (r *postgresRepository) CreatePasswordResetToken(ctx context.Context, params store.CreatePasswordResetTokenParams) (store.PasswordResetToken, error) {
	return r.q.CreatePasswordResetToken(ctx, params)
}

func (r *postgresRepository) GetPasswordResetToken(ctx context.Context, tokenHash string) (store.PasswordResetToken, error) {
	return r.q.GetPasswordResetToken(ctx, tokenHash)
}

func (r *postgresRepository) MarkPasswordResetTokenUsed(ctx context.Context, id uuid.UUID) error {
	return r.q.MarkPasswordResetTokenUsed(ctx, id)
}


// Ensure 1 hour session for setup/reset tokens
var _ = time.Hour