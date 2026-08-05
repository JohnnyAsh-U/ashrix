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
.PHONY: sqlc run-cp run-gateway

run-cp:
	$(AIR) -c .air.cp.toml

run-gateway:
	$(AIR) -c .air.gateway.toml

run-connector:
	$(AIR) -c .air.connector.toml



# ----- Generate Sql queries store --------

sqlc:
	cd internal/cp/database && $(SQLC) generate


# --- Generate Proto files ----------
.PHONY: proto

proto:
	buf generate


#====================================
# INSPECT BBOLT DB
#===================================
.PHONY: db
db-inspect:
	go run ./cmd/gateway/main.go db inspect

# ----- Generate Docs --------------

.PHONY: docs

docs-cp:
	swag init -g cmd/cp/main.go -o cmd/cp/docs --parseDependency --parseInternal

docs-gateway:
	swag init -g cmd/gateway/main.go -o cmd/gateway/docs --parseDependency --parseInternal

docs: docs-cp docs-gateway


#---------------Build PIPELINE--------------------------------
build-connector:
	GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui" -o build/windows/ashrix-connector.exe ./cmd/connector
	GOOS=linux GOARCH=amd64 go build -o build/linux/amd64/ashrix-connector ./cmd/connector
	GOOS=linux GOARCH=arm64 go build -o build/linux/arm64/ashrix-connector ./cmd/connector


