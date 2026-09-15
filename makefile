# Makefile
# -----Database configuration ------
DATABASE_URL ?= postgres://postgres:12345678@localhost:5432/ashrixdb?sslmode=disable

MODULE := $(shell go list -m)


#------Module----------------------
CP_VERSION_PKG := $(MODULE)/internal/cp/platform/version
BUILD_DATE := $(shell date -u +%Y%m%d-%H%M%S)
CP_LDFLAGS := -ldflags "-X $(CP_VERSION_PKG).Version=v1.0.0 \
					   -X $(CP_VERSION_PKG).BuildDate=$(BUILD_DATE)"



#------Gateway----------------------
GATEWAY_VERSION_PKG := $(MODULE)/internal/gateway/version
GATEWAY_BUILD_DATE := $(shell date -u +%Y%m%d-%H%M%S)
GATEWAY_LDFLAGS := -ldflags "-X $(GATEWAY_VERSION_PKG).Version=v1.0.0 \
					   -X $(GATEWAY_VERSION_PKG).BuildDate=$(BUILD_DATE)"


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


#---------------Build CP--------------------------------
.PHONY: build-cp
build-cp:
	@echo "Building with CP"
	go build $(CP_LDFLAGS) ./cmd/cp



#---------------Build PIPELINE--------------------------------
build-connector:
	GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui" -o build/windows/ashrix-connector.exe ./cmd/connector
	GOOS=linux GOARCH=amd64 go build -o build/linux/amd64/ashrix-connector ./cmd/connector
	GOOS=linux GOARCH=arm64 go build -o build/linux/arm64/ashrix-connector ./cmd/connector



#---------------Build Gateway--------------------------------
.PHONY: build-gateway
build-gateway:
	@echo "Building Gateway"
	go build $(GATEWAY_LDFLAGS) ./cmd/gateway



