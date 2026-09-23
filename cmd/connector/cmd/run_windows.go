//go:build windows

package cmd

import (
	"log"

	"golang.org/x/sys/windows/svc"
)

type connectorService struct{}

func (m *connectorService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	// Start runStart in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- runStart()
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				break loop
			}
		case err := <-errCh:
			if err != nil {
				log.Printf("Connector stopped with error: %v", err)
			}
			break loop
		}
	}

	return
}

func runServiceOrConsole() error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		log.Printf("failed to determine if we are running in an interactive session: %v", err)
		return runStart()
	}
	if isService {
		return svc.Run("AshrixConnector", &connectorService{})
	}
	return runStart()
}
