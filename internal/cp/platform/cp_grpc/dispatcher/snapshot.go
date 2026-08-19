package dispatcher

import "github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"

func CompactGatewayEvents(
	events []store.GatewayEvent,
) []store.GatewayEvent {

	if len(events) <= 1 {
		return events
	}

	latest := make(map[string]int)

	for i, event := range events {

		if event.DeliveryMode !=
			string(DeliverySnapshot) {

			continue
		}

		latest[event.Command] = i
	}

	result := make(
		[]store.GatewayEvent,
		0,
		len(events),
	)

	for i, event := range events {

		if event.DeliveryMode ==
			string(DeliverySnapshot) {

			latestIndex, ok :=
				latest[event.Command]

			if ok && latestIndex != i {
				continue
			}
		}

		result = append(result, event)
	}

	return result
}