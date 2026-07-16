//go:build linux

package tray

import "github.com/JohnnyAsh-U/ashrix-api/internal/connector/storage"

func runPlatform(s storage.Storage) {
	// Linux doesn't have system tray support in this implementation
	// Users should use CLI commands
}
