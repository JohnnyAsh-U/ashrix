//go:build !linux

package main

import "fmt"

func readPassword(string) (string, error) {
	return "", fmt.Errorf("interactive password bootstrap is currently supported on Linux only")
}
