//go:build windows

package tray

import (
	// "fmt"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/storage"
	"github.com/getlantern/systray"
	// "github.com/spf13/viper"
	"log"
	// "os/exec"
	// "time"
)

var (
	Storage        storage.Storage
	statusItem     *systray.MenuItem
	credStatusItem *systray.MenuItem
	startItem      *systray.MenuItem
	stopItem       *systray.MenuItem
	restartItem    *systray.MenuItem
	tokenItem      *systray.MenuItem
)

func runPlatform(s storage.Storage) {
	Storage = s
	systray.Run(onReady, onExit)
}


func onReady() {
    systray.SetTitle("Ashrix Connector")
    systray.SetTooltip("Ashrix Connector - Click to manage")

    statusItem = systray.AddMenuItem("Status: ⏹ Stopped", "Connection status")
    statusItem.Disable()

    credStatusItem = systray.AddMenuItem("🔑 Credential: ❌ Not found", "Credential status")
    credStatusItem.Disable()

    // updateStatus()
    // updateCredentialStatus()

    systray.AddSeparator()

    startItem = systray.AddMenuItem("▶ Start", "Start the connector")
    stopItem = systray.AddMenuItem("⏹ Stop", "Stop the connector")
    restartItem = systray.AddMenuItem("🔄 Restart", "Restart with stored credential")

    systray.AddSeparator()

    tokenItem = systray.AddMenuItem("🔑 Start with New Token", "Enter a new ephemeral token")

    systray.AddSeparator()

    configItem := systray.AddMenuItem("⚙️ Open Config", "Open config file")
    logsItem := systray.AddMenuItem("📋 Open Logs", "Open log directory")

    systray.AddSeparator()

    quitItem := systray.AddMenuItem("✖ Quit", "Exit application")

	_ = configItem
	_ = logsItem
	_ = quitItem
    // go handleEvents(startItem, stopItem, restartItem, tokenItem, configItem, logsItem, quitItem)
    // go monitorStatus()
}


// func handleEvents(start, stop, restart, token, config, logs, quit *systray.MenuItem) {
//     for {
//         select {
//         case <-start.ClickedCh:
//             go func() {
//                 log.Println("▶ Starting connector from tray...")
//                 if connector.IsRunning() {
//                     log.Println("ℹ️ Connector is already running")
//                     statusItem.SetTitle("Status: ✅ Already running")
//                     return
//                 }
//                 handleStartWithStoredCredential()
//             }()

//         case <-stop.ClickedCh:
//             go func() {
//                 if connector.IsRunning() {
//                     log.Println("⏹ Stopping connector from tray...")
//                     connector.Stop()
//                     updateStatus()
//                     log.Println("✅ Connector stopped")
//                 } else {
//                     log.Println("ℹ️ Connector is not running")
//                 }
//             }()

//         case <-restart.ClickedCh:
//             go func() {
//                 log.Println("🔄 Restarting connector from tray...")
//                 if connector.IsRunning() {
//                     connector.Stop()
//                     time.Sleep(1 * time.Second)
//                 }
//                 handleStartWithStoredCredential()
//             }()

//         case <-token.ClickedCh:
//             go func() {
//                 log.Println("🔑 Opening token input dialog...")
//                 tokenValue := ShowTokenDialog()

//                 if tokenValue == "" {
//                     log.Println("❌ No token provided, cancelled")
//                     return
//                 }

//                 log.Println("✅ Token received from dialog")

//                 if connector.IsRunning() {
//                     connector.Stop()
//                     time.Sleep(1 * time.Second)
//                 }

//                 log.Println("🔄 Registering with Control Plane...")
//                 cred, err := connector.RegisterWithCP(tokenValue)
//                 if err != nil {
//                     log.Printf("❌ Registration failed: %v", err)
//                     ShowErrorDialog("Registration failed:\n" + err.Error())
//                     return
//                 }

//                 if err := Storage.SaveCredential(cred); err != nil {
//                     log.Printf("❌ Failed to save credential: %v", err)
//                     ShowErrorDialog("Failed to save credential:\n" + err.Error())
//                     return
//                 }

//                 log.Printf("✅ Registered! Tunnel: %s", cred.TunnelID)
//                 updateCredentialStatus()

//                 connector.Start(cred)
//                 updateStatus()

//                 ShowInfoDialog(fmt.Sprintf(
//                     "✅ Connector started successfully!\n\n"+
//                         "Tunnel ID: %s\n"+
//                         "Credential expires: %s\n\n"+
//                         "⚠️ The ephemeral token is NOT stored.\n"+
//                         "You will need a new token for the next registration.",
//                     cred.TunnelID,
//                     cred.ExpiresAt.Format("2006-01-02 15:04:05"),
//                 ))
//             }()

//         case <-config.ClickedCh:
//             configPath := viper.ConfigFileUsed()
//             if configPath != "" {
//                 exec.Command("notepad", configPath).Start()
//             }

//         case <-logs.ClickedCh:
//             exec.Command("explorer", Storage.GetConfigDir()).Start()

//         case <-quit.ClickedCh:
//             if connector.IsRunning() {
//                 connector.Stop()
//             }
//             systray.Quit()
//         }
//     }
// }



// func handleStartWithStoredCredential() {
//     cred, err := Storage.LoadCredential()
//     if err != nil {
//         log.Println("❌ No credential found")
//         statusItem.SetTitle("Status: ❌ No credential")
//         credStatusItem.SetTitle("🔑 Credential: ❌ Not found")

//         ShowInfoDialog(`No credential found.

// To start, you need a new ephemeral token.
// Click "Start with New Token" from the tray menu.`)
//         return
//     }

//     if cred.IsExpired() {
//         log.Println("❌ Credential expired")
//         statusItem.SetTitle("Status: ❌ Expired")
//         credStatusItem.SetTitle("🔑 Credential: ❌ Expired")

//         ShowInfoDialog(fmt.Sprintf(`Credential expired on %s.

// You need a new ephemeral token.
// Click "Start with New Token" from the tray menu.`,
//             cred.ExpiresAt.Format("2006-01-02 15:04:05")))
//         return
//     }

//     remaining := time.Until(cred.ExpiresAt)
//     log.Printf("✅ Credential valid (expires in %s)", remaining.Round(time.Minute))

//     connector.Start(cred)
//     updateStatus()
//     updateCredentialStatus()
// }

// func updateStatus() {
//     if connector.IsRunning() {
//         statusItem.SetTitle("Status: ✅ Running")
//         statusItem.SetTooltip("Connector is running")
//     } else {
//         if Storage.CredentialExists() {
//             cred, _ := Storage.LoadCredential()
//             if cred != nil && !cred.IsExpired() {
//                 statusItem.SetTitle("Status: ⏹ Stopped")
//                 statusItem.SetTooltip("Connector stopped - click Start to run")
//             } else if cred != nil && cred.IsExpired() {
//                 statusItem.SetTitle("Status: ❌ Expired")
//                 statusItem.SetTooltip("Credential expired - get new token")
//             } else {
//                 statusItem.SetTitle("Status: ❌ No credential")
//                 statusItem.SetTooltip("No credential - start with new token")
//             }
//         } else {
//             statusItem.SetTitle("Status: ❌ No credential")
//             statusItem.SetTooltip("No credential - start with new token")
//         }
//     }
// }

// func updateCredentialStatus() {
//     if !Storage.CredentialExists() {
//         credStatusItem.SetTitle("🔑 Credential: ❌ Not found")
//         credStatusItem.SetTooltip("No credential stored - need new token")
//         return
//     }

//     cred, err := Storage.LoadCredential()
//     if err != nil {
//         credStatusItem.SetTitle("🔑 Credential: ❌ Error")
//         return
//     }

//     if cred.IsExpired() {
//         credStatusItem.SetTitle("🔑 Credential: ❌ Expired")
//         credStatusItem.SetTooltip(fmt.Sprintf("Expired on %s - get new token",
//             cred.ExpiresAt.Format("2006-01-02 15:04:05")))
//         return
//     }

//     remaining := time.Until(cred.ExpiresAt)
//     credStatusItem.SetTitle("🔑 Credential: ✅ Valid")
//     credStatusItem.SetTooltip(fmt.Sprintf("Tunnel: %s\nExpires: %s\nValid for: %s",
//         cred.TunnelID,
//         cred.ExpiresAt.Format("2006-01-02 15:04:05"),
//         remaining.Round(time.Minute)))
// }

// func monitorStatus() {
//     ticker := time.NewTicker(5 * time.Second)
//     for range ticker.C {
//         updateCredentialStatus()
//         if connector.IsRunning() {
//             statusItem.SetTitle("Status: ✅ Running")
//         }
//     }
// }

func onExit() {
    log.Println("Tray exited")
}