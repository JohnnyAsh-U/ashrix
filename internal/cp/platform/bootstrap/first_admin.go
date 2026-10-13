package bootstrap

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func CreateFirstAdmin(ctx context.Context, db *pgxpool.Pool, orgName, orgSlug, email,customDomain, password string) error {
	orgName = strings.TrimSpace(orgName)
	orgSlug = strings.TrimSpace(orgSlug)
	email = strings.TrimSpace(strings.ToLower(email))
	if orgName == "" || orgSlug == "" || email == "" || password == "" || customDomain=="" {
		return fmt.Errorf("organization name, slug, admin email, and password are required")
	}
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || parsedEmail.Address != email {
		return fmt.Errorf("admin email is invalid")
	}
	if len(password) < 4 || len(password) > 72 {
		return fmt.Errorf("password must contain 12 to 72 bytes")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin bootstrap transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('ashrix-first-admin-bootstrap'))`); err != nil {
		return fmt.Errorf("lock initial bootstrap: %w", err)
	}
	var orgCount, adminCount int64
	if err := tx.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM orgs WHERE deleted_at IS NULL),
		       (SELECT count(*) FROM admins WHERE revoked_at IS NULL)
	`).Scan(&orgCount, &adminCount); err != nil {
		return fmt.Errorf("check existing organizations and administrators: %w", err)
	}
	if orgCount != 0 || adminCount != 0 {
		return fmt.Errorf("initial bootstrap refused: found %d organizations and %d administrators", orgCount, adminCount)
	}

	var orgID string
	if err := tx.QueryRow(ctx, `INSERT INTO orgs (name, slug, custom_domain, domain_verified) VALUES ($1, $2, $3, $4) RETURNING id`, orgName, orgSlug, customDomain, true).Scan(&orgID); err != nil {
		return fmt.Errorf("create initial organization: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO admins (org_id, email, password_hash, role, is_active, otp_enabled)
		VALUES ($1, $2, $3, 'owner', true, false)
	`, orgID, email, string(passwordHash)); err != nil {
		return fmt.Errorf("create initial administrator: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit initial bootstrap: %w", err)
	}
	return nil
}
