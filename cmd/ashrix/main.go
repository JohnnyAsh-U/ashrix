package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/JohnnyAsh-U/ashrix-api/internal/client/auth"
	"github.com/JohnnyAsh-U/ashrix-api/internal/client/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/client/output"
	"github.com/JohnnyAsh-U/ashrix-api/internal/client/proxy"
	"github.com/JohnnyAsh-U/ashrix-api/internal/client/resource"
	"github.com/JohnnyAsh-U/ashrix-api/internal/client/ssh"
	"github.com/JohnnyAsh-U/ashrix-api/internal/client/storage"
	"github.com/JohnnyAsh-U/ashrix-api/internal/client/transport"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	store := storage.NewSecureStore()
	authSvc := auth.NewAuthService(cfg, store)
	resClient := resource.NewResourceClient(cfg, store)
	gwClient := transport.NewGatewayClient(cfg, store)

	logLevel := slog.LevelInfo
	if cfg.Debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	switch cmd {
	case "login":
		loginFlags := flag.NewFlagSet("login", flag.ExitOnError)
		email := loginFlags.String("email", "", "User email")
		password := loginFlags.String("password", "", "User password")
		_ = loginFlags.Parse(args)

		creds, err := authSvc.Login(ctx, *email, *password)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Login successful! Authenticated session saved.\n")
		if creds.UserEmail != "" {
			fmt.Printf("User: %s\n", creds.UserEmail)
		}

	case "logout":
		if err := authSvc.Logout(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Logout error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Logged out successfully. Session credentials removed.")

	case "whoami":
		creds, err := authSvc.GetCurrentSession()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		output.PrintWhoami(os.Stdout, creds)

	case "resources":
		resources, err := resClient.ListResources(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to fetch resources: %v\n", err)
			os.Exit(1)
		}
		output.PrintResources(os.Stdout, resources)

	case "connect":
		connFlags := flag.NewFlagSet("connect", flag.ExitOnError)
		localPort := connFlags.String("local-port", "0", "Local TCP port to listen on")
		_ = connFlags.Parse(args)

		if connFlags.NArg() < 1 {
			fmt.Fprintf(os.Stderr, "Usage: ashrix connect <resource> [--local-port <port>]\n")
			os.Exit(1)
		}
		targetResource := connFlags.Arg(0)

		res, err := resClient.ResolveResource(ctx, targetResource)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to resolve resource: %v\n", err)
			os.Exit(1)
		}

		p, err := proxy.NewLocalProxy(cfg, gwClient, res.ID, *localPort, logger)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create local proxy: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Forwarding local connections on %s -> Ashrix Resource %s\n", p.LocalAddr(), targetResource)
		fmt.Printf("Press Ctrl+C to terminate connection.\n")

		if err := p.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Proxy error: %v\n", err)
			os.Exit(1)
		}

	case "ssh":
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: ashrix ssh <resource> [ssh-options]\n")
			os.Exit(1)
		}
		targetResource := args[0]
		extraArgs := args[1:]

		if err := ssh.RunSSH(ctx, cfg, targetResource, extraArgs); err != nil {
			fmt.Fprintf(os.Stderr, "SSH error: %v\n", err)
			os.Exit(1)
		}

	case "ssh-proxy":
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: ashrix ssh-proxy <resource>\n")
			os.Exit(1)
		}
		targetResource := args[0]

		if err := ssh.RunSSHProxy(ctx, cfg, gwClient, targetResource); err != nil {
			os.Exit(1)
		}

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`Ashrix CLI - Zero Trust Access Client

Usage:
  ashrix <command> [arguments]

Commands:
  login                        Authenticate with Ashrix Control Plane
  logout                       Clear stored Ashrix session
  whoami                       Display current user session information
  resources                    List accessible private resources
  connect <resource> [--port]  Bridge local TCP port to Ashrix resource
  ssh <resource>               Connect to SSH resource via system OpenSSH
  ssh-proxy <resource>         Internal SSH ProxyCommand byte bridge
`)
}

