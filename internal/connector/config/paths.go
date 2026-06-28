package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func BaseDir() string {
	switch runtime.GOOS {
	case "windows":

		//ServiceMode: %ProgramData\Ashrix%
		if pd := os.Getenv("ProgramData"); pd != "" && isService() {
			return filepath.Join(pd, "Ashrix")
		}
		dir, _ := os.UserConfigDir()
		return filepath.Join(dir, "Ashrix")
	case "linux", "darwin":
		//Root/service /var/lib/ashrix
		//User: ~/.config/ashrix
		if os.Geteuid() == 0 {
			return "/var/lib/ashrix"
		}

		dir, _ := os.UserConfigDir()

		return filepath.Join(dir, "ashrix")

	default:
		dir, _ := os.UserConfigDir()
		return filepath.Join(dir, "ashrix")
	}
}

// Check check: if no console and running as system/root
func isService() bool {
	return runtime.GOOS == "windows" && os.Getenv("SESSIONNAME") == ""
}

func InitDirs() error {
	certDir := filepath.Join(BaseDir(), "certs")
	logDir := filepath.Join(BaseDir(), "logs")

	//Create the each folder; may exist or may not
	// Create certs directory
	err := os.MkdirAll(certDir, 0700)
	if err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Create logs directory
	err = os.MkdirAll(logDir, 0700)
	if err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	return nil
}
