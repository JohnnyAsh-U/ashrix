# Implementation Plan - Ashrix CLI Client

We need to implement the **Ashrix CLI client** for the Zero Trust private-resource access platform. The CLI must remain a thin, untrusted client that connects to the Control Plane (CP) for authentication and resource discovery, and connects to the Gateway via QUIC/TLS (with authenticated session credential) for transparent byte-stream forwarding to private resources behind Connectors.

## 1. Key Technical Architectural Decisions

1. **Architecture & Trust Domain**:
   - **CP (HTTPS)**: CLI authenticates via OIDC browser flow / OAuth PKCE or CP session endpoint, fetching session tokens and user accessible resources.
   - **Gateway (QUIC/TLS)**: Human user CLI connects to Gateway using server TLS authentication + session credential (NOT mTLS with client certs). Uses `pkg/frame` to send `StreamFrame` for opening logical streams, then uses `pkg/flow.Relay` for transparent bidirectional proxying.
   - **Untrusted Client**: The Gateway enforces authorization, session validity, tenant context, and policy. CLI parameters (user ID, tenant ID, role, etc.) are NOT trusted for authorization.

2. **Reuse Existing Project Code**:
   - `pkg/frame`: Length-prefixed framing (`WriteFrame`, `ReadFrame`, `DecodeFrame`) using `proto.StreamFrame`.
   - `pkg/flow`: `Relay` helper for bidirectional streaming and half-close EOF semantics.
   - `proto`: `proto.StreamFrame` (`RequestId`, `SessionId`, `DestAppId`, `DestAppName`, `FlowType = USER_TO_APP`, `ProtocolType`).

3. **Command Structure**:
   - `ashrix login`: Interactive OIDC browser login / local PKCE server callback or login session flow with CP; stores tokens securely.
   - `ashrix logout`: Clears saved session credentials from keyring/storage and notifies CP if necessary.
   - `ashrix whoami`: Displays current logged-in user profile / session state.
   - `ashrix resources`: Fetches and lists accessible resources from CP.
   - `ashrix connect <resource> [--local-port 15432]`: Local TCP listener on `127.0.0.1` bridging local TCP connections over QUIC stream to Gateway.
   - `ashrix ssh <resource>`: Invokes system `ssh` with `-o ProxyCommand="ashrix ssh-proxy <resource>"`.
   - `ashrix ssh-proxy <resource>`: Internal command reading stdin -> writing to QUIC stream, reading QUIC stream -> writing to stdout. ALL diagnostics go to stderr.

4. **Credential Storage**:
   - Primary: OS Keyring / Secret Service / Keychain (using `github.com/zalando/go-keyring` or fallback file storage with secure 0600 permissions in `~/.config/ashrix/credentials.json`).
   - Config file in `~/.config/ashrix/config.yaml` storing `control_plane_url`, `gateway_url`, etc.

## 2. Proposed Component Directory Structure

We will implement the CLI client under `internal/client/` and `cmd/ashrix/` (or `cmd/client/` to fit repository conventions):

- `internal/client/config`: Configuration loading (`config.yaml`, environment variables) & default settings.
- `internal/client/auth`: CP Authentication, OIDC PKCE flow / local HTTP callback server, session token management.
- `internal/client/storage`: Credential storage (OS Keychain / keyring abstraction with file fallback).
- `internal/client/resource`: Resource discovery client interacting with CP API.
- `internal/client/transport`: QUIC client wrapper over Gateway (`quic.DialAddr`), establishing QUIC connections and opening multiplexed `quic.Stream`s wrapped with `pkg/frame` handshakes.
- `internal/client/proxy`: Local TCP proxy (`LocalProxy`) listening on `127.0.0.1` and bridging to QUIC streams.
- `internal/client/ssh`: OpenSSH invocation wrapper (`ashrix ssh`) and stdio stream bridge (`ashrix ssh-proxy`).
- `internal/client/output`: Formatted stdout/stderr printing (tabwriter for `resources`, structured stderr for `ssh-proxy`).
- `cmd/ashrix/main.go` (and `cmd/client/main.go` wrapper): CLI entrypoint parsing arguments and dispatching subcommands.

---

## 3. Detailed Component Plan

### [NEW] `internal/client/config/config.go`
- Manages `control_plane_url`, `gateway_url`, `quic_port`, `debug`.
- Reads from `~/.config/ashrix/config.yaml` or env vars (`ASHRIX_CP_URL`, `ASHRIX_GATEWAY_URL`).

### [NEW] `internal/client/storage/keyring.go`
- Store & retrieve access token / session token.
- Secure fallback file storage (`~/.config/ashrix/session.json`, `0600` mode) if OS keyring is unavailable.

### [NEW] `internal/client/auth/auth.go`
- `ashrix login`: Spins up local PKCE callback server (`http://127.0.0.1:<random-port>/callback`), opens browser to CP auth URL, exchanges authorization code for Ashrix session token.
- `ashrix logout`: Invalidates local credentials and calls CP logout endpoint.
- `ashrix whoami`: Reads current user info from token / CP `/auth/me`.

### [NEW] `internal/client/resource/resource.go`
- `ashrix resources`: Calls CP endpoint `/v1/resources` (or equivalent API endpoint) using session token to fetch list of authorized resources (`ID`, `Name`, `Type`, `Destination`, `Port`, `Status`).

### [NEW] `internal/client/transport/quic_client.go`
- Establishes QUIC connection to Gateway using TLS server verification.
- Implements stream opening with `pkg/frame.WriteFrame` using `proto.StreamFrame` (`FlowType: USER_TO_APP`, `SessionId`, `DestAppId/DestAppName`).
- Handles response ACK / ERR frame reading.
- Wraps `quic.Stream` into a `flow.Stream` interface for transparent byte relaying.

### [NEW] `internal/client/proxy/local_proxy.go`
- `LocalProxy` listens on `127.0.0.1:<port>`.
- Accepts incoming local TCP connections (e.g. from `psql`, `redis-cli`, browser).
- For each connection, opens an authorized QUIC stream to Gateway, and runs `flow.Relay(ctx, localConn, quicStream)`.

### [NEW] `internal/client/ssh/ssh.go` & `ssh_proxy.go`
- `ashrix ssh-proxy <resource>`:
  - Connects to Gateway via QUIC, sends `StreamFrame` for target SSH resource.
  - Reads from `os.Stdin`, writes to QUIC stream; reads from QUIC stream, writes to `os.Stdout`.
  - All logs/errors MUST be printed to `os.Stderr`.
- `ashrix ssh <resource>`:
  - Finds self executable path (`os.Executable()`).
  - Executes system `ssh` binary with argument `-o ProxyCommand="<executable> ssh-proxy <resource>" <user>@<host>`.

### [NEW] `cmd/client/main.go` (and `cmd/ashrix/main.go`)
- Parses subcommands (`login`, `logout`, `whoami`, `resources`, `connect`, `ssh`, `ssh-proxy`).
- Executes corresponding command handler with proper context and error logging.

---

## 4. Verification Plan

### Automated Tests
- Unit tests for configuration loading (`internal/client/config`).
- Unit tests for credential storage & fallback mechanism (`internal/client/storage`).
- Unit tests for `LocalProxy` TCP listener and bidirectional stream forwarding using `net.Pipe()`.
- Unit tests for `ssh-proxy` stream piping ensuring stderr isolation and clean EOF propagation.
- Test verifying CLI-supplied user identity/roles/tenant metadata are ignored server-side and server session is authoritative.

### Verification Commands
- `go test ./internal/client/...`
- Build CLI: `go build -o bin/ashrix ./cmd/client`
- Run `bin/ashrix --help` to verify all subcommands are present.
