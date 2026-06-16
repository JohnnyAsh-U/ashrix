# Makefile
# -----Database configuration ------
DATABASE_URL ?= postgres://postgres:12345678@localhost:5432/ashrixdb?sslmode=disable

# ------ Paths and tools ---------
MIGRATIONS_DIR = internal/cp/database/migrations
GOOSE = goose
AIR = air
SQLC = sqlc

# ----- Migration commands --------
.PHONY: migrate-up migrate-down migrate-status migrate-create

migrate-up:
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres $(DATABASE_URL) up

migrate-down:
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres $(DATABASE_URL) down

migrate-status:
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres $(DATABASE_URL) status

migrate-create:
	$(GOOSE) -dir $(MIGRATIONS_DIR) create $(NAME) sql

#------- Reset DB -------
.PHONY: reset-db

reset-db:
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres $(DATABASE_URL) down
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres $(DATABASE_URL) up


# ----- Development run using Air -------
.PHONY: sqlc run

run:
	$(AIR)

# ----- Generate Sql queries store --------

sqlc:
	cd internal/cp/database && $(SQLC) generate


# --- Generate Proto files ----------
.PHONY: proto

proto:
	buf generate


# ----- Generate Docs --------------

.PHONY: docs

docs-cp:
	swag init -g cmd/cp/main.go -o cmd/cp/docs --parseDependency --parseInternal

docs-gateway:
	swag init -g cmd/gateway/main.go -o cmd/gateway/docs --parseDependency --parseInternal

docs: docs-cp docs-gateway