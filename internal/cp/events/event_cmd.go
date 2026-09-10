package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GatewayEvent struct {
	GatewayID string

	Seq int64

	EventID string

	Command CommandType

	DeliveryMode DeliveryMode

	Payload []byte

	CreatedAt time.Time
}

func GatewayEventToCmd(
	event store.GatewayEvent,
) (*gen.Command, error) {

	job := CommandJob{
		Type:      CommandType(event.Command),
		GatewayID: event.GatewayID.String(),
	}

	if len(event.Payload) > 0 {
		var err error

		job, err = UnmarshalCommand(
			event.Payload,
		)

		if err != nil {
			return nil, err
		}
	}

	return BuildCommand(job)
}

func BuildCommand(
	job CommandJob,
) (*gen.Command, error) {

	switch job.Type {

	case CmdRotateGatewayCert:
		return &gen.Command{
			Payload: &gen.Command_RotateGatewayCert{
				RotateGatewayCert: &gen.RotateGatewayCertCmd{},
			},
		}, nil

	case CmdRevokeGatewayCert:
		return &gen.Command{
			Payload: &gen.Command_RevokeGatewayCert{
				RevokeGatewayCert: &gen.RevokeGatewayCertCmd{},
			},
		}, nil

	case CmdRevokeGateway:
		return &gen.Command{
			Payload: &gen.Command_RevokeGateway{
				RevokeGateway: &gen.RevokeGatewayCmd{},
			},
		}, nil

	case CmdReloadConnector:
		return &gen.Command{
			Payload: &gen.Command_ReloadConnector{
				ReloadConnector: &gen.ReloadConnectorCmd{
					ConnectorId: job.ConnectorID,
				},
			},
		}, nil

	case CmdDrainGateway:
		return &gen.Command{
			Payload: &gen.Command_DrainGateway{
				DrainGateway: &gen.DrainGatewayCmd{},
			},
		}, nil

	case CmdRotateConnectorCert:
		return &gen.Command{
			Payload: &gen.Command_RotateConnectorCert{
				RotateConnectorCert: &gen.RotateConnectorCertCmd{
					ConnectorId: job.ConnectorID,
				},
			},
		}, nil

	case CmdRevokeConnectorCert:
		return &gen.Command{
			Payload: &gen.Command_RevokeConnectorCert{
				RevokeConnectorCert: &gen.RevokeConnectorCertCmd{
					ConnectorId: job.ConnectorID,
				},
			},
		}, nil

	case CmdRevokeConnector:
		return &gen.Command{
			Payload: &gen.Command_RevokeConnector{
				RevokeConnector: &gen.RevokeConnectorCmd{
					ConnectorId: job.ConnectorID,
				},
			},
		}, nil

	case CmdRevokeUserSession:
		return &gen.Command{
			Payload: &gen.Command_RevokeSession{
				RevokeSession: &gen.RevokeSessionCmd{
					SessionId:        job.SessionID,
					SessionExpiresAt: timestamppb.New(job.SessionExpires),
				},
			},
		}, nil

	case CmdCrlSync:
		return &gen.Command{
			Payload: &gen.Command_CrlSync{
				CrlSync: &gen.CrlSyncCmd{
					RevokedSerialNumbers: job.RevokedSerialNumbers,
				},
			},
		}, nil

	case CmdConnectorSync:
		return &gen.Command{
			Payload: &gen.Command_ConnectorSync{
				ConnectorSync: &gen.ConnectorSyncCmd{
					Connectors: job.ConnectorInfo,
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf(
			"unsupported command type: %s",
			job.Type,
		)
	}
}

func MarshalCommand(job CommandJob) ([]byte, error) {
	return json.Marshal(
		persistedCommand{
			Type:                 job.Type,
			GatewayID:            job.GatewayID,
			ConnectorID:          job.ConnectorID,
			SessionID:            job.SessionID,
			RevokedSerialNumbers: job.RevokedSerialNumbers,
			SessionExpires:       job.SessionExpires,
			ConnectorInfo:        job.ConnectorInfo,
		},
	)
}

func UnmarshalCommand(data []byte) (CommandJob, error) {
	var command persistedCommand

	if err := json.Unmarshal(data, &command); err != nil {
		return CommandJob{}, fmt.Errorf(
			"unmarshal command payload: %w",
			err,
		)
	}

	return CommandJob{
		Type:                 command.Type,
		GatewayID:            command.GatewayID,
		ConnectorID:          command.ConnectorID,
		SessionID:            command.SessionID,
		RevokedSerialNumbers: command.RevokedSerialNumbers,
		SessionExpires:       command.SessionExpires,
		ConnectorInfo:        command.ConnectorInfo,
	}, nil
}
