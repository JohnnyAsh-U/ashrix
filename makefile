# ------ Paths and tools ---------
AIR = air



# ----- Development run using Air -------
.PHONY: run-gateway

run-gateway:
	$(AIR) -c .air.gateway.toml

run-connector:
	$(AIR) -c .air.connector.toml


# --- Generate Proto files ----------
.PHONY: proto

proto:
	buf generate


#---------------Build PIPELINE--------------------------------
build-connector:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o build/windows/ashrix-connector.exe ./cmd/connector
	GOOS=linux GOARCH=amd64 go build -o build/linux/amd64/ashrix-connector ./cmd/connector
	GOOS=linux GOARCH=arm64 go build -o build/linux/arm64/ashrix-connector ./cmd/connector



#---------------Build Gateway--------------------------------
.PHONY: build-gateway
build-gateway:
	@echo "Building Gateway"
	go build $(GATEWAY_LDFLAGS) ./cmd/gateway



