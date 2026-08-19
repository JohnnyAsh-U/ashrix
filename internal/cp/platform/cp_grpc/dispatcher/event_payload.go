package dispatcher

import (
	"encoding/json"
	"fmt"

	gen "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

type persistedCommand struct {
	Type CommandType `json:"type"`

	GatewayID string `json:"gateway_id"`

	ConnectorID string `json:"connector_id,omitempty"`

	SessionID string `json:"session_id,omitempty"`

	RevokedSerialNumbers []string `json:"revoked_serial_numbers,omitempty"`

	ConnectorInfo []*gen.ConnectorInfo `json:"connector_info,omitempty"`
}

func MarshalCommand(job CommandJob) ([]byte, error) {
	return json.Marshal(
		persistedCommand{
			Type:                  job.Type,
			GatewayID:             job.GatewayID,
			ConnectorID:           job.ConnectorID,
			SessionID:             job.SessionID,
			RevokedSerialNumbers:  job.RevokedSerialNumbers,
			ConnectorInfo:         job.ConnectorInfo,
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
		Type:                  command.Type,
		GatewayID:             command.GatewayID,
		ConnectorID:           command.ConnectorID,
		SessionID:             command.SessionID,
		RevokedSerialNumbers:  command.RevokedSerialNumbers,
		ConnectorInfo:         command.ConnectorInfo,
	}, nil
}