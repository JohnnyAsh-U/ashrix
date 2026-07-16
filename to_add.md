// internal/ipc/ipc.go

package ipc

// Commands the tray/CLI sends to the service
type Command struct {
    Action string `json:"action"` // "status", "stop", "restart"
}

// StatusResponse is what the service sends back
type StatusResponse struct {
    State       string `json:"state"`        // "connected", "disconnected", "connecting"
    Transport   string `json:"transport"`     // "quic", "websocket"
    ConnectorID string `json:"connector_id"`
    TenantID    string `json:"tenant_id"`
    GatewayAddr string `json:"gateway_addr"`
    UptimeSeconds int64 `json:"uptime_seconds"`
    AppAddr     string `json:"app_addr"`
    LastError   string `json:"last_error,omitempty"`
}




// internal/platform/windows/service.go
//go:build windows

package windows

import (
    "context"
    "time"

    "golang.org/x/sys/windows/svc"
    "golang.org/x/sys/windows/svc/debug"
    "golang.org/x/sys/windows/svc/eventlog"
    "go.uber.org/zap"

    "github.com/JohnnyAsh-U/ashrix-api/internal/connector"
)

const serviceName = "AshrixConnector"

type windowsService struct {
    cfg connector.Config
    log *zap.Logger
}

// Run is called by the Windows Service Control Manager.
func (s *windowsService) Execute(
    args []string,
    changeRequests <-chan svc.ChangeRequest,
    status chan<- svc.Status,
) (bool, uint32) {

    // Tell SCM we are starting
    status <- svc.Status{State: svc.StartPending}

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Start connector in background
    conn := connector.New(s.cfg, s.log)
    connErrCh := make(chan error, 1)
    go func() {
        connErrCh <- conn.Run(ctx)
    }()

    // Tell SCM we are running
    status <- svc.Status{
        State:   svc.Running,
        Accepts: svc.AcceptStop | svc.AcceptShutdown,
    }

    // Handle SCM commands
    for {
        select {
        case req := <-changeRequests:
            switch req.Cmd {
            case svc.Stop, svc.Shutdown:
                status <- svc.Status{State: svc.StopPending}
                cancel()
                return false, 0
            }
        case err := <-connErrCh:
            s.log.Error("connector exited", zap.Error(err))
            return true, 1 // true = report error to SCM
        }
    }
}

// InstallService registers the connector as a Windows service.
func InstallService(exePath string) error {
    m, err := mgr.Connect()
    if err != nil {
        return err
    }
    defer m.Disconnect()

    s, err := m.CreateService(
        serviceName,
        exePath,
        mgr.Config{
            DisplayName: "Ashrix Connector",
            Description: "Ashrix secure application access connector",
            StartType:   mgr.StartAutomatic, // start on boot
        },
        "run", // arguments passed to service
    )
    if err != nil {
        return err
    }
    defer s.Close()

    // Start immediately after install
    return s.Start()
}

// StartAsService runs the binary as a Windows service.
// Called when SCM starts the process.
func StartAsService(cfg connector.Config, log *zap.Logger) error {
    return svc.Run(serviceName, &windowsService{cfg: cfg, log: log})
}




// internal/platform/windows/tray.go
//go:build windows

package windows

import (
    "time"

    "github.com/getlantern/systray"
    "go.uber.org/zap"

    "github.com/JohnnyAsh-U/ashrix-api/internal/ipc"
    _ "embed"
)

//go:embed icons/connected.ico
var iconConnected []byte

//go:embed icons/disconnected.ico
var iconDisconnected []byte

//go:embed icons/connecting.ico
var iconConnecting []byte

// RunTray starts the system tray icon and menu.
// Blocks until tray exits.
func RunTray(log *zap.Logger) {
    systray.Run(
        func() { onTrayReady(log) },
        func() { onTrayExit(log) },
    )
}

func onTrayReady(log *zap.Logger) {
    systray.SetIcon(iconConnecting)
    systray.SetTitle("Ashrix Connector")
    systray.SetTooltip("Ashrix Connector — connecting...")

    // ── Menu items ─────────────────────────────────────────────────
    mStatus  := systray.AddMenuItem("⬤ Connecting...", "Current status")
    mStatus.Disable() // status is display only, not clickable

    systray.AddSeparator()

    mRestart := systray.AddMenuItem("Restart", "Restart the connector")
    mStop    := systray.AddMenuItem("Stop", "Stop the connector")
    mStart   := systray.AddMenuItem("Start", "Start the connector")
    mStart.Hide() // hidden until connector is stopped

    systray.AddSeparator()

    mLogs    := systray.AddMenuItem("View Logs", "Open log file")
    mAbout   := systray.AddMenuItem("About Ashrix", "Version info")

    systray.AddSeparator()

    mQuit    := systray.AddMenuItem("Quit Tray", "Close tray icon (connector keeps running)")

    // ── Status poller ──────────────────────────────────────────────
    // Poll connector service every 5s via IPC
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()

        for range ticker.C {
            status := getStatus()
            updateTrayStatus(status, mStatus, mStop, mStart)
        }
    }()

    // ── Menu click handlers ────────────────────────────────────────
    go func() {
        for {
            select {
            case <-mRestart.ClickedCh:
                sendCommand("restart")
                mStatus.SetTitle("⬤ Restarting...")
                systray.SetTooltip("Ashrix Connector — restarting...")
                systray.SetIcon(iconConnecting)

            case <-mStop.ClickedCh:
                sendCommand("stop")
                mStatus.SetTitle("⬤ Stopped")
                systray.SetIcon(iconDisconnected)
                systray.SetTooltip("Ashrix Connector — stopped")
                mStop.Hide()
                mStart.Show()

            case <-mStart.ClickedCh:
                sendCommand("start")
                mStatus.SetTitle("⬤ Connecting...")
                systray.SetIcon(iconConnecting)
                systray.SetTooltip("Ashrix Connector — connecting...")
                mStart.Hide()
                mStop.Show()

            case <-mLogs.ClickedCh:
                openLogsFolder()

            case <-mAbout.ClickedCh:
                showAboutDialog()

            case <-mQuit.ClickedCh:
                systray.Quit()
                return
            }
        }
    }()
}

func onTrayExit(log *zap.Logger) {
    log.Info("tray exited")
}

func updateTrayStatus(
    status *ipc.StatusResponse,
    mStatus *systray.MenuItem,
    mStop *systray.MenuItem,
    mStart *systray.MenuItem,
) {
    switch status.State {
    case "connected":
        systray.SetIcon(iconConnected)
        systray.SetTooltip("Ashrix Connector — connected")
        mStatus.SetTitle("✓ Connected via " + status.Transport)
        mStop.Show()
        mStart.Hide()

    case "disconnected":
        systray.SetIcon(iconDisconnected)
        systray.SetTooltip("Ashrix Connector — disconnected")
        if status.LastError != "" {
            mStatus.SetTitle("✗ Disconnected: " + status.LastError)
        } else {
            mStatus.SetTitle("✗ Disconnected")
        }
        mStop.Hide()
        mStart.Show()

    case "connecting":
        systray.SetIcon(iconConnecting)
        systray.SetTooltip("Ashrix Connector — connecting...")
        mStatus.SetTitle("⬤ Connecting...")
    }
}

func getStatus() *ipc.StatusResponse {
    // Talk to connector service via named pipe
    resp, err := ipc.SendCommand("status")
    if err != nil {
        return &ipc.StatusResponse{State: "disconnected", LastError: err.Error()}
    }
    return resp
}

func sendCommand(action string) {
    ipc.SendCommand(action)
}

func openLogsFolder() {
    // Open %PROGRAMDATA%\Ashrix\logs in Explorer
    exec.Command("explorer", logDir()).Start()
}

func showAboutDialog() {
    // Simple about box using fyne or windows dialog
    dialog.ShowInfo(
        "Ashrix Connector",
        "Version 1.0.0\nSecure application access\nashrix.io",
    )
}


// internal/platform/windows/setup.go
//go:build windows

package windows

import (
    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/app"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/widget"
    "fyne.io/fyne/v2/theme"
    "go.uber.org/zap"
)

// ShowSetupWindow shows the token entry window on first install.
// Blocks until user enters a valid token and connects,
// or cancels.
func ShowSetupWindow(log *zap.Logger) error {
    a := app.New()
    w := a.NewWindow("Ashrix Connector Setup")
    w.Resize(fyne.NewSize(420, 280))
    w.SetFixedSize(true)
    w.CenterOnScreen()

    // ── UI elements ───────────────────────────────────────────────
    logo := widget.NewLabel("🔐 Ashrix Connector")
    logo.TextStyle = fyne.TextStyle{Bold: true}

    subtitle := widget.NewLabel("Enter the token provided by your IT administrator")
    subtitle.Wrapping = fyne.TextWrapWord

    tokenEntry := widget.NewPasswordEntry()
    tokenEntry.SetPlaceHolder("ct_xxxxxxxxxxxxxxxx")

    statusLabel := widget.NewLabel("")
    statusLabel.Wrapping = fyne.TextWrapWord

    connectBtn := widget.NewButton("Connect", nil)
    connectBtn.Importance = widget.HighImportance // primary button style

    cancelBtn := widget.NewButton("Cancel", func() {
        a.Quit()
    })

    // ── Connect button logic ───────────────────────────────────────
    connectBtn.OnTapped = func() {
        token := tokenEntry.Text
        if token == "" {
            statusLabel.SetText("⚠ Please enter a token")
            return
        }

        connectBtn.Disable()
        connectBtn.SetText("Connecting...")
        statusLabel.SetText("Registering with Ashrix...")

        // Run registration in background — never block the UI goroutine
        go func() {
            err := registerWithToken(token, log)
            if err != nil {
                // Update UI from main goroutine
                fyne.CurrentApp().Driver().CanvasForObject(w.Canvas())
                statusLabel.SetText("✗ " + err.Error())
                connectBtn.SetText("Try Again")
                connectBtn.Enable()
                return
            }

            // Success
            statusLabel.SetText("✓ Connected! Ashrix is running in your system tray.")
            connectBtn.SetText("Done")
            connectBtn.OnTapped = func() { a.Quit() }
            connectBtn.Enable()
            cancelBtn.Hide()
        }()
    }

    // Allow Enter key to trigger connect
    tokenEntry.OnSubmitted = func(_ string) {
        connectBtn.OnTapped()
    }

    // ── Layout ─────────────────────────────────────────────────────
    content := container.NewVBox(
        container.NewPadded(logo),
        subtitle,
        widget.NewSeparator(),
        widget.NewLabel("Connector Token:"),
        tokenEntry,
        statusLabel,
        container.NewHBox(
            cancelBtn,
            connectBtn,
        ),
    )

    w.SetContent(container.NewPadded(content))
    w.ShowAndRun() // blocks until window closes

    return nil
}

// registerWithToken calls the connector registration flow.
// This is what happens invisibly after the user hits Connect.
func registerWithToken(token string, log *zap.Logger) error {
    // 1. Call CP with token to get connector credentials
    // 2. Save credentials to disk
    // 3. Install and start Windows service
    // 4. Verify service is running and connected

    log.Info("registering with token")

    creds, err := bootstrap.RegisterConnector(token, cpURL())
    if err != nil {
        return fmt.Errorf("registration failed: %v", err)
    }

    if err := creds.SaveToDisk(dataDir()); err != nil {
        return fmt.Errorf("failed to save credentials: %v", err)
    }

    if err := InstallService(executablePath()); err != nil {
        return fmt.Errorf("failed to install service: %v", err)
    }

    // Wait for service to come online (max 15s)
    for i := 0; i < 15; i++ {
        time.Sleep(1 * time.Second)
        status := getStatus()
        if status.State == "connected" {
            return nil
        }
    }

    return fmt.Errorf("service installed but not connecting — check network")
}






#!/bin/bash
# install.sh — Ashrix Connector installer for Linux

set -e

ASHRIX_VERSION="1.0.0"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/etc/ashrix"
LOG_DIR="/var/log/ashrix"
SERVICE_USER="ashrix"
BINARY_URL="https://releases.ashrix.io/connector/linux/amd64/ashrix-connector"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()    { echo -e "${GREEN}✓${NC} $1"; }
warn()    { echo -e "${YELLOW}⚠${NC} $1"; }
fatal()   { echo -e "${RED}✗${NC} $1"; exit 1; }

echo ""
echo "  Ashrix Connector Installer v${ASHRIX_VERSION}"
echo "  ──────────────────────────────────"
echo ""

# ── Root check ────────────────────────────────────────────────────────────────
if [ "$EUID" -ne 0 ]; then
    fatal "Please run as root: sudo bash install.sh"
fi

# ── Get token ─────────────────────────────────────────────────────────────────
if [ -z "$1" ]; then
    echo -n "  Enter connector token: "
    read -r TOKEN
else
    TOKEN="$1"
fi

if [ -z "$TOKEN" ]; then
    fatal "Token is required"
fi

# ── Download binary ───────────────────────────────────────────────────────────
echo ""
info "Downloading Ashrix Connector..."
curl -sSL "$BINARY_URL" -o "$INSTALL_DIR/ashrix-connector"
chmod +x "$INSTALL_DIR/ashrix-connector"

# ── Create user and directories ───────────────────────────────────────────────
info "Creating service user..."
useradd --system --no-create-home --shell /bin/false "$SERVICE_USER" 2>/dev/null || true

info "Creating directories..."
mkdir -p "$DATA_DIR" "$LOG_DIR"
chown "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR" "$LOG_DIR"
chmod 750 "$DATA_DIR" "$LOG_DIR"

# ── Register connector ────────────────────────────────────────────────────────
info "Registering connector with Ashrix..."
ashrix-connector register --token="$TOKEN" --data-dir="$DATA_DIR"
if [ $? -ne 0 ]; then
    fatal "Registration failed — check token and network connectivity"
fi

# ── Install systemd service ───────────────────────────────────────────────────
info "Installing systemd service..."
cat > /etc/systemd/system/ashrix-connector.service << EOF
[Unit]
Description=Ashrix Connector
Documentation=https://docs.ashrix.io/connector
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
ExecStart=${INSTALL_DIR}/ashrix-connector start --data-dir=${DATA_DIR}
Restart=on-failure
RestartSec=5s
StartLimitInterval=60s
StartLimitBurst=5

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${DATA_DIR} ${LOG_DIR}

# Environment
Environment=ASHRIX_ENV=prod
Environment=ASHRIX_LOG_DIR=${LOG_DIR}

[Install]
WantedBy=multi-user.target
EOF

# ── Enable and start ──────────────────────────────────────────────────────────
info "Starting service..."
systemctl daemon-reload
systemctl enable ashrix-connector
systemctl start ashrix-connector

# ── Verify ────────────────────────────────────────────────────────────────────
echo ""
info "Waiting for connector to come online..."
sleep 3

STATUS=$(ashrix-connector status --format=json 2>/dev/null | jq -r '.state' 2>/dev/null || echo "unknown")

if [ "$STATUS" = "connected" ]; then
    echo ""
    echo -e "  ${GREEN}✓ Ashrix Connector is running and connected${NC}"
    echo ""
    ashrix-connector status
else
    echo ""
    warn "Service started but not yet connected (state: $STATUS)"
    warn "Check logs: journalctl -u ashrix-connector -f"
fi

echo ""
echo "  Commands:"
echo "    ashrix-connector status"
echo "    ashrix-connector stop"
echo "    ashrix-connector start"
echo "    ashrix-connector restart"
echo "    ashrix-connector logs"
echo ""






// cmd/status.go

var statusCmd = &cobra.Command{
    Use:   "status",
    Short: "Show connector status",
    RunE: func(cmd *cobra.Command, args []string) error {
        format, _ := cmd.Flags().GetString("format")

        status, err := ipc.SendCommand("status")
        if err != nil {
            // Service not running
            if format == "json" {
                json.NewEncoder(os.Stdout).Encode(map[string]string{
                    "state": "stopped",
                    "error": err.Error(),
                })
                return nil
            }
            fmt.Println("● ashrix-connector — stopped")
            return nil
        }

        if format == "json" {
            return json.NewEncoder(os.Stdout).Encode(status)
        }

        // Human readable output
        stateIcon := map[string]string{
            "connected":    "●",
            "disconnected": "○",
            "connecting":   "◌",
        }[status.State]

        fmt.Printf("\n  %s ashrix-connector — %s\n\n", stateIcon, status.State)
        fmt.Printf("  Connector ID : %s\n", status.ConnectorID)
        fmt.Printf("  Tenant       : %s\n", status.TenantID)
        fmt.Printf("  Transport    : %s\n", status.Transport)
        fmt.Printf("  App          : %s\n", status.AppAddr)
        fmt.Printf("  Uptime       : %s\n", formatUptime(status.UptimeSeconds))

        if status.LastError != "" {
            fmt.Printf("  Last Error   : %s\n", status.LastError)
        }

        fmt.Println()
        return nil
    },
}

// cmd/control.go — stop, start, restart share same pattern

var stopCmd = &cobra.Command{
    Use:   "stop",
    Short: "Stop the connector",
    RunE: func(cmd *cobra.Command, args []string) error {
        if runtime.GOOS == "linux" {
            return exec.Command("systemctl", "stop", "ashrix-connector").Run()
        }
        return ipc.SendCommand("stop")
    },
}

var startCmd = &cobra.Command{
    Use:   "start",
    Short: "Start the connector",
    RunE: func(cmd *cobra.Command, args []string) error {
        if runtime.GOOS == "linux" {
            return exec.Command("systemctl", "start", "ashrix-connector").Run()
        }
        return ipc.SendCommand("start")
    },
}

var restartCmd = &cobra.Command{
    Use:   "restart",
    Short: "Restart the connector",
    RunE: func(cmd *cobra.Command, args []string) error {
        if runtime.GOOS == "linux" {
            return exec.Command("systemctl", "restart", "ashrix-connector").Run()
        }
        return ipc.SendCommand("restart")
    },
}

var logsCmd = &cobra.Command{
    Use:   "logs",
    Short: "Show connector logs",
    RunE: func(cmd *cobra.Command, args []string) error {
        follow, _ := cmd.Flags().GetBool("follow")

        if runtime.GOOS == "linux" {
            journalArgs := []string{"-u", "ashrix-connector", "--no-pager"}
            if follow {
                journalArgs = append(journalArgs, "-f")
            }
            c := exec.Command("journalctl", journalArgs...)
            c.Stdout = os.Stdout
            c.Stderr = os.Stderr
            return c.Run()
        }

        // Windows — tail the log file
        return tailLogFile(logDir(), follow)
    },
}







; installer.nsi — NSIS installer script
; Compiles to AshrixConnectorSetup.exe

!include "MUI2.nsh"

Name "Ashrix Connector"
OutFile "AshrixConnectorSetup.exe"
InstallDir "$PROGRAMFILES64\Ashrix\Connector"
RequestExecutionLevel admin

; Icons
!define MUI_ICON "assets\ashrix.ico"
!define MUI_UNICON "assets\ashrix.ico"

; Pages
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

Section "Install"
    SetOutPath "$INSTDIR"

    ; Copy binaries
    File "build\windows\ashrix-connector.exe"
    File "build\windows\ashrix-tray.exe"
    File "assets\ashrix.ico"

    ; Create data directory
    CreateDirectory "$APPDATA\Ashrix\Connector"
    CreateDirectory "$APPDATA\Ashrix\Logs"

    ; Add to startup (tray app)
    WriteRegStr HKCU \
        "Software\Microsoft\Windows\CurrentVersion\Run" \
        "AshrixTray" \
        '"$INSTDIR\ashrix-tray.exe"'

    ; Write uninstaller
    WriteUninstaller "$INSTDIR\Uninstall.exe"

    ; Add to Programs list
    WriteRegStr HKLM \
        "Software\Microsoft\Windows\CurrentVersion\Uninstall\AshrixConnector" \
        "DisplayName" "Ashrix Connector"
    WriteRegStr HKLM \
        "Software\Microsoft\Windows\CurrentVersion\Uninstall\AshrixConnector" \
        "UninstallString" "$INSTDIR\Uninstall.exe"

    ; Launch setup window for token entry
    Exec '"$INSTDIR\ashrix-tray.exe" --setup'
SectionEnd

Section "Uninstall"
    ; Stop and remove service
    ExecWait 'sc stop AshrixConnector'
    ExecWait 'sc delete AshrixConnector'

    ; Remove startup entry
    DeleteRegValue HKCU \
        "Software\Microsoft\Windows\CurrentVersion\Run" \
        "AshrixTray"

    ; Remove files
    RMDir /r "$INSTDIR"
    RMDir /r "$APPDATA\Ashrix"
SectionEnd









# Makefile

build-windows:
	GOOS=windows GOARCH=amd64 \
	go build -o build/windows/ashrix-connector.exe ./cmd/connector
	GOOS=windows GOARCH=amd64 \
	go build -o build/windows/ashrix-tray.exe ./cmd/tray
	# Compile NSIS installer
	makensis installer.nsi

build-linux-amd64:
	GOOS=linux GOARCH=amd64 \
	go build -o build/linux/amd64/ashrix-connector ./cmd/connector

build-linux-arm64:
	GOOS=linux GOARCH=arm64 \
	go build -o build/linux/arm64/ashrix-connector ./cmd/connector

release: build-windows build-linux-amd64 build-linux-arm64
	# Upload to releases.ashrix.io








AshrixConnectorSetup.exe  (Inno Setup)
  │
  ├── Extracts files to %PROGRAMFILES%\Ashrix\Connector\
  │     ashrix-connector.exe   ← Go service binary
  │     ashrix-tray.exe        ← Go tray binary (embeds WebView2 window)
  │     WebView2Loader.dll     ← WebView2 runtime loader
  │
  ├── Installs WebView2 Runtime (if not present)
  │
  ├── Launches ashrix-tray.exe --setup
  │     ↓
  │     Opens WebView2 window (HTML/CSS/JS)
  │     User enters token
  │     JS calls Go via webview2 bindings
  │     Go registers connector, installs service
  │     JS shows success/error state
  │
  └── Registers ashrix-tray.exe in startup




// cmd/tray/setup.go
//go:build windows

package main

import (
    "encoding/json"
    "fmt"

    webview "github.com/jchv/go-webview2"
    "go.uber.org/zap"
)

// ShowSetupWindow opens the WebView2 token entry window.
// Blocks until user completes setup or closes window.
func ShowSetupWindow(log *zap.Logger) error {
    w := webview.NewWithOptions(webview.WebViewOptions{
        Debug:     false,
        AutoFocus: true,
        WindowOptions: webview.WindowOptions{
            Title:  "Ashrix Connector Setup",
            Width:  480,
            Height: 360,
            Center: true,
            Frameless: false,
        },
    })
    if w == nil {
        return fmt.Errorf("failed to create WebView2 window")
    }
    defer w.Destroy()

    // ── Bind Go functions callable from JavaScript ─────────────────
    // JS calls: window.ashrix.connect(token)
    // Go executes registration, returns result to JS
    w.Bind("ashrixConnect", func(token string) string {
        err := registerWithToken(token, log)
        if err != nil {
            result, _ := json.Marshal(map[string]interface{}{
                "ok":    false,
                "error": err.Error(),
            })
            return string(result)
        }
        result, _ := json.Marshal(map[string]interface{}{
            "ok": true,
        })
        return string(result)
    })

    // JS calls: window.ashrix.getStatus()
    // Returns current connector status
    w.Bind("ashrixGetStatus", func() string {
        status := getConnectorStatus()
        result, _ := json.Marshal(status)
        return string(result)
    })

    // JS calls: window.ashrix.openDocs()
    // Opens help page in default browser
    w.Bind("ashrixOpenDocs", func() {
        openURL("https://docs.ashrix.io/connector/setup")
    })

    // ── Load the HTML UI ───────────────────────────────────────────
    // Embedded at build time — no external files needed
    w.SetHtml(setupHTML)

    w.Run()
    return nil
}






// cmd/tray/setup_html.go
//go:build windows

package main

// setupHTML is embedded in the binary at build time.
// No external HTML files needed.
const setupHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Ashrix Connector Setup</title>
<style>
  * {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  body {
    font-family: -apple-system, 'Segoe UI', sans-serif;
    background: #0f1117;
    color: #e2e8f0;
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    user-select: none;
  }

  .card {
    background: #1a1d2e;
    border: 1px solid #2d3148;
    border-radius: 12px;
    padding: 36px;
    width: 400px;
  }

  .logo {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 24px;
  }

  .logo-icon {
    width: 32px;
    height: 32px;
    background: #6366f1;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 16px;
  }

  .logo-text {
    font-size: 18px;
    font-weight: 600;
    color: #f1f5f9;
  }

  .subtitle {
    font-size: 13px;
    color: #94a3b8;
    margin-bottom: 28px;
    line-height: 1.5;
  }

  label {
    display: block;
    font-size: 12px;
    font-weight: 500;
    color: #94a3b8;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-bottom: 8px;
  }

  input {
    width: 100%;
    padding: 10px 14px;
    background: #0f1117;
    border: 1px solid #2d3148;
    border-radius: 8px;
    color: #f1f5f9;
    font-size: 14px;
    font-family: 'Consolas', monospace;
    outline: none;
    transition: border-color 0.15s;
  }

  input:focus {
    border-color: #6366f1;
  }

  input::placeholder {
    color: #475569;
  }

  .btn {
    width: 100%;
    padding: 11px;
    margin-top: 16px;
    background: #6366f1;
    color: white;
    border: none;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    transition: background 0.15s, opacity 0.15s;
  }

  .btn:hover { background: #4f46e5; }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .status {
    margin-top: 16px;
    padding: 12px 14px;
    border-radius: 8px;
    font-size: 13px;
    display: none;
    align-items: center;
    gap: 10px;
  }

  .status.connecting {
    display: flex;
    background: #1e293b;
    color: #94a3b8;
    border: 1px solid #2d3148;
  }

  .status.success {
    display: flex;
    background: #052e16;
    color: #4ade80;
    border: 1px solid #166534;
  }

  .status.error {
    display: flex;
    background: #2d0a0a;
    color: #f87171;
    border: 1px solid #7f1d1d;
  }

  .spinner {
    width: 16px;
    height: 16px;
    border: 2px solid #334155;
    border-top-color: #6366f1;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    flex-shrink: 0;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .help {
    margin-top: 20px;
    text-align: center;
    font-size: 12px;
    color: #475569;
  }

  .help a {
    color: #6366f1;
    text-decoration: none;
    cursor: pointer;
  }

  .help a:hover { text-decoration: underline; }
</style>
</head>
<body>

<div class="card">

  <div class="logo">
    <div class="logo-icon">🔐</div>
    <span class="logo-text">Ashrix Connector</span>
  </div>

  <p class="subtitle">
    Enter the connector token provided by your administrator
    to securely connect this machine to your organization.
  </p>

  <label for="token">Connector Token</label>
  <input
    type="password"
    id="token"
    placeholder="ct_xxxxxxxxxxxxxxxx"
    autocomplete="off"
    spellcheck="false"
  />

  <button class="btn" id="connectBtn" onclick="connect()">
    Connect
  </button>

  <div class="status connecting" id="statusConnecting">
    <div class="spinner"></div>
    <span id="connectingMsg">Registering with Ashrix...</span>
  </div>

  <div class="status success" id="statusSuccess">
    <span>✓</span>
    <span>Connected. Ashrix is running in your system tray.</span>
  </div>

  <div class="status error" id="statusError">
    <span>✗</span>
    <span id="errorMsg">Connection failed.</span>
  </div>

  <div class="help">
    Need help? <a onclick="openDocs()">View setup guide</a>
  </div>

</div>

<script>
  const tokenInput   = document.getElementById('token')
  const connectBtn   = document.getElementById('connectBtn')
  const statusConn   = document.getElementById('statusConnecting')
  const statusOK     = document.getElementById('statusSuccess')
  const statusErr    = document.getElementById('statusError')
  const connectingMsg = document.getElementById('connectingMsg')
  const errorMsg     = document.getElementById('errorMsg')

  // Allow Enter key to submit
  tokenInput.addEventListener('keydown', e => {
    if (e.key === 'Enter') connect()
  })

  // Focus token input on load
  window.onload = () => tokenInput.focus()

  function hideAll() {
    statusConn.style.display = 'none'
    statusOK.style.display   = 'none'
    statusErr.style.display  = 'none'
  }

  async function connect() {
    const token = tokenInput.value.trim()
    if (!token) {
      showError('Please enter a connector token')
      return
    }

    // Disable UI during connection
    connectBtn.disabled = true
    tokenInput.disabled = true
    hideAll()

    // Show connecting state
    connectingMsg.textContent = 'Registering with Ashrix...'
    statusConn.style.display = 'flex'

    try {
      // Call Go function bound via webview2
      // ashrixConnect is bound in setup.go
      const resultJSON = await window.ashrixConnect(token)
      const result = JSON.parse(resultJSON)

      if (result.ok) {
        hideAll()
        statusOK.style.display = 'flex'
        connectBtn.textContent = 'Done'
        connectBtn.disabled = false
        connectBtn.onclick = () => window.close()
        tokenInput.disabled = false
      } else {
        showError(result.error)
        tokenInput.disabled = false
        connectBtn.disabled = false
        connectBtn.textContent = 'Try Again'
      }

    } catch (err) {
      showError('Unexpected error — please try again')
      tokenInput.disabled = false
      connectBtn.disabled = false
      connectBtn.textContent = 'Try Again'
    }
  }

  function showError(msg) {
    hideAll()
    errorMsg.textContent = msg
    statusErr.style.display = 'flex'
  }

  function openDocs() {
    window.ashrixOpenDocs()
  }
</script>

</body>
</html>
`









; ashrix-connector.iss
; Compile with: iscc ashrix-connector.iss

#define AppName "Ashrix Connector"
#define AppVersion "1.0.0"
#define AppPublisher "Ashrix"
#define AppURL "https://ashrix.io"
#define AppExeName "ashrix-connector.exe"
#define TrayExeName "ashrix-tray.exe"
#define ServiceName "AshrixConnector"

[Setup]
AppId={{YOUR-GUID-HERE}}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppURL}
DefaultDirName={autopf}\Ashrix\Connector
DefaultGroupName=Ashrix
OutputDir=dist
OutputBaseFilename=AshrixConnectorSetup
SetupIconFile=assets\ashrix.ico
UninstallDisplayIcon={app}\ashrix.ico
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
; Minimum Windows 10
MinVersion=10.0.17763

; Don't show components page — silent install
DisableWelcomePage=no
DisableProgramGroupPage=yes
DisableReadyPage=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "startup"; \
  Description: "Start Ashrix Connector when Windows starts"; \
  GroupDescription: "Additional options:"; \
  Flags: checked

[Files]
; Main binaries
Source: "build\windows\{#AppExeName}";   DestDir: "{app}"; Flags: ignoreversion
Source: "build\windows\{#TrayExeName}";  DestDir: "{app}"; Flags: ignoreversion
Source: "assets\ashrix.ico";             DestDir: "{app}"; Flags: ignoreversion

; WebView2 runtime loader
Source: "build\windows\WebView2Loader.dll"; DestDir: "{app}"; Flags: ignoreversion

; WebView2 runtime bootstrapper (installs Edge WebView2 if missing)
Source: "assets\MicrosoftEdgeWebview2Setup.exe"; \
  DestDir: "{tmp}"; \
  Flags: deleteafterinstall

[Icons]
; Start menu
Name: "{group}\Ashrix Connector";     Filename: "{app}\{#TrayExeName}"
Name: "{group}\Uninstall Ashrix";     Filename: "{uninstallexe}"

[Registry]
; Tray auto-start
Root: HKCU; \
  Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; \
  ValueType: string; \
  ValueName: "AshrixTray"; \
  ValueData: """{app}\{#TrayExeName}"""; \
  Tasks: startup; \
  Flags: uninsdeletevalue

[Run]
; 1. Install WebView2 runtime silently if not present
Filename: "{tmp}\MicrosoftEdgeWebview2Setup.exe"; \
  Parameters: "/silent /install"; \
  Check: WebView2NotInstalled; \
  StatusMsg: "Installing WebView2 runtime..."; \
  Flags: waituntilterminated

; 2. Launch setup window for token entry
; --setup flag triggers WebView2 window instead of normal tray
Filename: "{app}\{#TrayExeName}"; \
  Parameters: "--setup"; \
  Description: "Connect to Ashrix now"; \
  Flags: nowait postinstall skipifsilent

[UninstallRun]
; Stop and remove service on uninstall
Filename: "{sys}\sc.exe"; Parameters: "stop {#ServiceName}";   Flags: waituntilterminated
Filename: "{sys}\sc.exe"; Parameters: "delete {#ServiceName}"; Flags: waituntilterminated

[Code]
// Check if WebView2 runtime is already installed
// Checks registry for Edge WebView2 installation
function WebView2NotInstalled(): Boolean;
var
  version: string;
begin
  Result := not RegQueryStringValue(
    HKLM,
    'SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}',
    'pv',
    version
  );
end;

// Custom status messages during install
procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssInstall then
  begin
    WizardForm.StatusLabel.Caption := 'Installing Ashrix Connector...';
  end;
end;

// Confirm before uninstall
function InitializeUninstall(): Boolean;
begin
  Result := MsgBox(
    'This will remove Ashrix Connector from your computer.' + #13#10 +
    'Your applications will no longer be accessible through Ashrix.' + #13#10#13#10 +
    'Are you sure?',
    mbConfirmation,
    MB_YESNO
  ) = IDYES;
end;









// cmd/tray/main.go
//go:build windows

package main

import (
    "os"

    "go.uber.org/zap"
)

func main() {
    log, _ := zap.NewProduction()
    defer log.Sync()

    // Inno Setup launches us with --setup on first install
    // Normal startup (from registry Run key) has no flags
    args := os.Args[1:]

    if len(args) > 0 && args[0] == "--setup" {
        // First run — show token entry window
        if err := ShowSetupWindow(log); err != nil {
            log.Fatal("setup window failed", zap.Error(err))
        }
        return
    }

    // Normal run — show tray icon
    RunTray(log)
}






1. Downloads AshrixConnectorSetup.exe (one file, ~15MB)

2. Double-clicks it
   UAC prompt: "Allow this app to make changes?" → Yes

3. Installer wizard:
   ┌─────────────────────────────────────┐
   │  Welcome to Ashrix Connector Setup  │
   │                                     │
   │  This will install Ashrix           │
   │  Connector v1.0.0 on your computer  │
   │                                     │
   │  [Next >]  [Cancel]                 │
   └─────────────────────────────────────┘

4. If WebView2 not installed:
   "Installing WebView2 runtime..."  (silent, 30 seconds)

5. Files copy:
   "Installing Ashrix Connector..."

6. Setup window opens (WebView2):
   ┌─────────────────────────────────────┐
   │ 🔐 Ashrix Connector                 │
   │                                     │
   │ Enter the connector token provided  │
   │ by your administrator...            │
   │                                     │
   │ CONNECTOR TOKEN                     │
   │ [••••••••••••••••••••••••••••••]   │
   │                                     │
   │ [        Connect        ]           │
   │                                     │
   │ Need help? View setup guide         │
   └─────────────────────────────────────┘

7. IT person pastes token, clicks Connect:
   ┌─────────────────────────────────────┐
   │ ◌ Registering with Ashrix...        │
   └─────────────────────────────────────┘

8. Success:
   ┌─────────────────────────────────────┐
   │ ✓ Connected. Ashrix is running      │
   │   in your system tray.              │
   │                                     │
   │ [          Done          ]          │
   └─────────────────────────────────────┘

9. System tray icon appears (green circle)
   Right-click shows:
   ● Connected via QUIC
   ───────────────────
   Restart
   Stop
   ───────────────────
   View Logs
   About Ashrix
   ───────────────────
   Quit Tray




# Makefile

INNO = "C:\Program Files (x86)\Inno Setup 6\ISCC.exe"

build-connector-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
	go build \
	  -ldflags="-H windowsgui" \
	  -o build/windows/ashrix-connector.exe \
	  ./cmd/connector

build-tray-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
	go build \
	  -ldflags="-H windowsgui" \
	  -o build/windows/ashrix-tray.exe \
	  ./cmd/tray

# CGO_ENABLED=1 required for WebView2
# -H windowsgui prevents console window appearing

build-installer: build-connector-windows build-tray-windows
	$(INNO) ashrix-connector.iss

build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	go build -o build/linux/ashrix-connector ./cmd/connector





The One Thing To Get Right Early
-H windowsgui in the linker flags. Without this, every time the tray app runs, a black console window flashes open for a split second. IT people will call support asking what the black window is.
Set it from day one.