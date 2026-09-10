package events

import (
	"time"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)


type CommandType string

const (
	CmdRotateGatewayCert CommandType = "ROTATE_GATEWAY_CERT"
	CmdRevokeGatewayCert CommandType = "REVOKE_GATEWAY_CERT"
	CmdRevokeGateway     CommandType = "REVOKE_GATEWAY"
	CmdDrainGateway      CommandType = "DRAIN_GATEWAY"

	CmdRotateConnectorCert CommandType = "ROTATE_CONNECTOR_CERT"
	CmdRevokeConnectorCert CommandType = "REVOKE_CONNECTOR_CERT"
	CmdRevokeConnector     CommandType = "REVOKE_CONNECTOR"
	CmdReloadConnector	   CommandType = "RELOAD_CONNECTOR"

	CmdRevokeUserSession CommandType = "REVOKE_USER_SESSION"
	CmdCrlSync           CommandType = "CRL_SYNC"
	CmdConnectorSync     CommandType = "CONNECTOR_SYNC"
)

type DeliveryMode string
const (
	DeliveryAction   DeliveryMode = "ACTION"
	DeliverySnapshot DeliveryMode = "SNAPSHOT"
)


type persistedCommand struct {
	Type CommandType `json:"type"`

	GatewayID string `json:"gateway_id"`

	ConnectorID string `json:"connector_id,omitempty"`

	SessionID string `json:"session_id,omitempty"`

	SessionExpires time.Time `json:"session_expires,omitempty"`

	RevokedSerialNumbers []string `json:"revoked_serial_numbers,omitempty"`

	ConnectorInfo []*proto.ConnectorInfo `json:"connector_info,omitempty"`
}


func DeliveryModeForCommand(cmd CommandType) DeliveryMode {
	switch cmd {
	case CmdCrlSync, CmdConnectorSync:
		return DeliverySnapshot
	default:
		return DeliveryAction
	}
}

type CommandJob struct {
	Type CommandType

	GatewayID string

	ConnectorID string

	SessionID string

	SessionExpires time.Time

	RevokedSerialNumbers []string

	ConnectorInfo []*proto.ConnectorInfo
}

