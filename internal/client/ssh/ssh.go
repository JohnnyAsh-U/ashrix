package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/JohnnyAsh-U/ashrix-api/internal/client/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/client/transport"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/flow"
)

type stdStream struct {
	r io.Reader
	w io.Writer
}

func (s *stdStream) Read(p []byte) (n int, err error) {
	return s.r.Read(p)
}

func (s *stdStream) Write(p []byte) (n int, err error) {
	return s.w.Write(p)
}

func (s *stdStream) Close() error {
	return nil
}

func (s *stdStream) CloseRead() error {
	return nil
}

func (s *stdStream) CloseWrite() error {
	return nil
}

func (s *stdStream) Context() context.Context {
	return context.Background()
}

type flowAdapter struct {
	io.ReadWriteCloser
}

func (f *flowAdapter) CloseRead() error {
	return nil
}

func (f *flowAdapter) CloseWrite() error {
	return f.Close()
}

func (f *flowAdapter) Context() context.Context {
	return context.Background()
}

// RunSSHProxy handles the `ashrix ssh-proxy <resource>` command.
// CRITICAL: stdout is the SSH data channel. All diagnostics/errors MUST go to stderr.
func RunSSHProxy(ctx context.Context, cfg *config.Config, gwClient *transport.GatewayClient, resourceID string) error {
	gwStream, err := gwClient.DialAndOpenStream(ctx, resourceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ashrix ssh-proxy error: %v\n", err)
		return err
	}
	defer gwStream.Close()

	streamStdinStdout := &stdStream{r: os.Stdin, w: os.Stdout}
	streamGW := &flowAdapter{ReadWriteCloser: gwStream}

	res := flow.Relay(ctx, streamStdinStdout, streamGW)
	if res.Err != nil && res.Err != io.EOF {
		fmt.Fprintf(os.Stderr, "ashrix ssh-proxy finished with error: %v\n", res.Err)
		return res.Err
	}

	return nil
}

// RunSSH handles the user-facing `ashrix ssh <resource>` command.
// It invokes system OpenSSH with -o ProxyCommand="<self> ssh-proxy <resource>".
func RunSSH(ctx context.Context, cfg *config.Config, resourceID string, additionalArgs []string) error {
	execPath, err := os.Executable()
	if err != nil {
		execPath = "ashrix"
	}

	proxyCmd := fmt.Sprintf("%s ssh-proxy %s", execPath, resourceID)

	sshArgs := []string{
		"-o", fmt.Sprintf("ProxyCommand=%s", proxyCmd),
		resourceID,
	}
	sshArgs = append(sshArgs, additionalArgs...)

	cmd := exec.CommandContext(ctx, "ssh", sshArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("failed to execute OpenSSH: %w", err)
	}

	return nil
}
