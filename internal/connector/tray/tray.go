package tray

import "github.com/JohnnyAsh-U/ashrix-api/internal/connector/storage"

// Run starts the system tray (Windows only, Linux stub)
func Run(storage storage.Storage) {
    // Platform-specific implementation in separate files
    runPlatform(storage)
}