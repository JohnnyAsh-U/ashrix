package main

import "os"

// import "github.com/JohnnyAsh-U/ashrix-api/cmd/gateway/cli"

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
