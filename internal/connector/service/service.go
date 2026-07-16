package service

import (
	"fmt"
	"runtime"
)

func Install() error {
	switch runtime.GOOS {
	case "windows":
		return install()
	case "linux":
		return install()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func Uninstall() error {
	switch runtime.GOOS {
	case "windows":
		return uninstall()
	case "linux":
		return uninstall()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}