package logging
// internal/logger/events.go

const (
    // Lifecycle
	EventGatewayRegistered   = "GATEWAY_REGISTERED"
    EventGatewayStart        = "GATEWAY_START"
    EventGatewayStop         = "GATEWAY_STOP"
    EventCPConnected         = "CP_CONNECTED"
    EventCPDisconnected      = "CP_DISCONNECTED"
    EventCPHelloAck          = "CP_HELLO_ACK"
    EventCPHelloRejected     = "CP_HELLO_REJECTED"

    // Connector lifecycle
    EventConnectorRegistered = "CONNECTOR_REGISTERED"
    EventConnectorRejected   = "CONNECTOR_REJECTED"
    EventConnectorRevoked    = "CONNECTOR_REVOKED"
    EventConnectorSuspended  = "CONNECTOR_SUSPENDED"
    EventConnectorDisconnect = "CONNECTOR_DISCONNECTED"

    // Access decisions
    EventAccessGrant         = "ACCESS_GRANT"
    EventAccessDeny          = "ACCESS_DENY"

    // Security
    EventTokenInvalid        = "TOKEN_INVALID"
    EventCertExpired         = "CERT_EXPIRED"
    EventCertRevoked         = "CERT_REVOKED"
    EventStreamLimitExceeded = "STREAM_LIMIT_EXCEEDED"
    EventUnknownTenant       = "UNKNOWN_TENANT"
    EventPolicyStale         = "POLICY_STALE"
)