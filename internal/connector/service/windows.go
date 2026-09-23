//go:build windows

package service

import (
	"log"
	"os"
	"golang.org/x/sys/windows/svc/mgr"
)

func install() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	s, err := m.CreateService(
		"AshrixConnector",
		exePath,
		mgr.Config{
			DisplayName: "Ashrix Connector",
			Description: "Manages Ashrix tunnel connections",
			StartType:   mgr.StartAutomatic,
		},
		"start",
	)
	if err != nil {
		return err
	}
	defer s.Close()

	log.Println("✅ Service 'AshrixConnector' installed!")
	return nil
}

func uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService("AshrixConnector")
	if err != nil {
		return err
	}
	defer s.Close()

	return s.Delete()
}