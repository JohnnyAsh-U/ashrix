package repositories

import (
	// "context"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/app"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/auth"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/connector"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/events"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/gateway"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/identity"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/logs"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/org"
	pkica "github.com/JohnnyAsh-U/ashrix-api/internal/cp/pki_ca"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/policy"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Import and initialize all the repositories here to abstract away the db from the code

type Repositories struct {
	Policy policy.Repository
	Gateway gateway.Repository
	Connector connector.Repository
	PKICA pkica.Repository
	App app.Repository
	Org org.Repository
	IDP identity.Repository
	Auth auth.Repository
	Event events.Repository
	Log logs.Repository
}


func NewRepositories(db *pgxpool.Pool, dbQueries *store.Queries) *Repositories {
	return &Repositories{
		Policy: policy.NewRepository(db, dbQueries),
		Gateway: gateway.NewPostgresRepository(dbQueries),
		Connector: connector.NewPostgresRepository(dbQueries),
		PKICA: pkica.NewPKICARepository(dbQueries),
		App: app.NewPostgresRepository(dbQueries),
		Org: org.NewPostgresRepository(dbQueries),
		IDP: identity.NewPostgresRepository(dbQueries),
		Auth: auth.NewPostgresRepository(dbQueries),
		Event: events.NewRepository(dbQueries, db),
		Log: logs.NewPostgresRepository(dbQueries),
	}
}


