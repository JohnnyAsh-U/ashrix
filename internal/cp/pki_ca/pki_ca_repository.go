package pkica

import (
	"context"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/google/uuid"
)

type Repository interface {
	GetActiveCACert(ctx context.Context, params store.GetActiveCACertParams) (store.GetActiveCACertRow, error)
	GetActiveComponentCert(ctx context.Context, params store.GetActiveComponentCertParams) (store.ComponentCertificate, error)
	CreateCACert(ctx context.Context, params store.InsertCACertParams) (uuid.UUID , error)
	GetActiveComponentCertByType(ctx context.Context, componentType string) (store.ComponentCertificate, error)
	CreateCRLEntry(ctx context.Context, params store.CreateCRLEntryParams) (store.CrlEntry, error)
	GetCRLEntry(ctx context.Context) ([]store.CrlEntry, error)
	GetCRLEntryBySerial(ctx context.Context, serialNumber string) (store.CrlEntry, error)
	CreateComponentCert(ctx context.Context, params store.RegisterCompCertParams) (store.ComponentCertificate, error)
	RevokeComponentCert(ctx context.Context, params store.RevokeCompCertParams) (store.ComponentCertificate, error)
}

type postgresRepository struct {
	q *store.Queries
}

func NewPKICARepository(q *store.Queries) Repository {
	return &postgresRepository{q: q}
}

func (r *postgresRepository) GetActiveCACert(ctx context.Context, params store.GetActiveCACertParams) (store.GetActiveCACertRow, error) {
	return r.q.GetActiveCACert(ctx, params)
}

func (r *postgresRepository) GetActiveComponentCert(ctx context.Context, params store.GetActiveComponentCertParams) (store.ComponentCertificate, error) {
	return r.q.GetActiveComponentCert(ctx, params)
}

func (r *postgresRepository) CreateCACert(ctx context.Context, params store.InsertCACertParams) (uuid.UUID, error) {
	return r.q.InsertCACert(ctx, params)
}

func (r *postgresRepository) GetActiveComponentCertByType(ctx context.Context, componentType string) (store.ComponentCertificate, error) {
	return r.q.GetActiveComponentCertByType(ctx, componentType)
}

func (r *postgresRepository) CreateCRLEntry(ctx context.Context, params store.CreateCRLEntryParams) (store.CrlEntry, error) {
	return r.q.CreateCRLEntry(ctx, params)
}

func (r *postgresRepository) GetCRLEntry(ctx context.Context) ([]store.CrlEntry, error) {
	return r.q.ListCRLEntries(ctx)
}

func (r *postgresRepository) GetCRLEntryBySerial(ctx context.Context, serialNumber string) (store.CrlEntry, error) {
	return r.q.GetCRLEntryBySerial(ctx, serialNumber)
}

func (r *postgresRepository) CreateComponentCert(ctx context.Context, params store.RegisterCompCertParams) (store.ComponentCertificate, error) {
	return r.q.RegisterCompCert(ctx, params)
}

func (r *postgresRepository) RevokeComponentCert(ctx context.Context, params store.RevokeCompCertParams) (store.ComponentCertificate, error) {
	return r.q.RevokeCompCert(ctx, params)
}



