//go:build linux

package tray

import (
	"log"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/storage"
)

func runPlatform(s storage.Storage) {
	log.Println("⚠️  System tray not available on Linux")
	log.Println("💡 Use CLI commands instead:")
	log.Println("   ashrix-connector list          # List connections")
	log.Println("   ashrix-connector connect <name>  # Connect")
	log.Println("   ashrix-connector disconnect <name> # Disconnect")
}
