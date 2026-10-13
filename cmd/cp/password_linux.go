//go:build linux

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	fd := int(os.Stdin.Fd())
	state, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return "", fmt.Errorf("password input requires an interactive terminal: %w", err)
	}
	updated := *state
	updated.Lflag &^= unix.ECHO
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &updated); err != nil {
		return "", fmt.Errorf("disable terminal echo: %w", err)
	}
	defer func() {
		_ = unix.IoctlSetTermios(fd, unix.TCSETS, state)
		fmt.Fprintln(os.Stderr)
	}()
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	if line == "" && err == io.EOF {
		return "", fmt.Errorf("no password entered")
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}
