# Ashrix integration tests

These tests exercise the real gateway and connector packages together over local TCP/UDP sockets. They generate short-lived test certificates and use local `httptest` applications; they do not require external services or production credentials.

The gateway, connector, and test applications run as separate component instances in the test process. These are component/network integration tests, not yet OS-process deployment tests.

## Coverage

- Gateway and connector establish a QUIC session using TLS 1.3 mutual certificate authentication and register the connector.
- HTTP requests traverse the gateway, QUIC stream, connector, and configured local HTTP application; method, path, query, request body, response status, response body, and response headers are checked.
- Multiple application hostnames on one connector route to their respective configured upstreams.
- Unknown hostnames return `404`; the internal health endpoint returns `200`.
- A configured route with no connected connector returns `503`.
- The HTTPS listener serves requests over TLS.
- Connector cancellation removes the active gateway session.
- Concurrent requests and multi-megabyte request bodies are transferred without corruption.
- WebSocket upgrade and bidirectional messages traverse the gateway/connector tunnel.
- A connector certificate issued by an untrusted CA is rejected during the QUIC TLS handshake.

## Run

From the repository root:

```bash
go test ./integration -v
```

Run the race detector and the complete unit + integration suite:

```bash
go test -race ./integration
go test ./...
go test -race ./...
go vet ./...
```

Coverage:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Boundaries

These tests do not replace a deployment test that starts the compiled `ashrix-gateway` and `ashrix-connector` binaries as independent OS processes with real YAML files, installed certificate files, and operator-managed startup/shutdown. That is a separate deployment acceptance test. Performance/load benchmarks are also separate from correctness integration tests.

On Linux, the suite also includes a process-level test that builds both CLI binaries, writes isolated YAML configurations and certificate files under temporary `HOME` directories, starts the gateway and connector as separate OS processes, verifies a real HTTP request through the tunnel, and terminates both processes with `SIGTERM`:

```bash
go test ./integration -run TestGatewayAndConnectorBinariesWorkTogetherFromYAML -v
```
