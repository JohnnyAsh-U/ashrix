//go:build windows

package tray

import (
	"fmt"
	// "log"
	"regexp"

	"github.com/lxn/walk"
	"github.com/sqweek/dialog"
)

// func ShowTokenDialog() string {
//     log.Println("🔑 Opening token input dialog...")

//     token, err := dialog.Entry("Enter your ephemeral token from the Control Plane:\n\nThe token is NOT stored - you need a new one each time.",
//         "Ashrix Connector - Token Input")

//     if err != nil {
//         log.Printf("❌ Dialog cancelled or error: %v", err)
//         return ""
//     }

//     if token == "" {
//         log.Println("❌ No token entered")
//         return ""
//     }

//     log.Println("✅ Token received from dialog")
//     return token
// }

func ShowInfoDialog(message string) {
    dialog.Message("%s", message).Title("Ashrix Connector").Info()
}

func ShowErrorDialog(message string) {
    dialog.Message("%s", message).Title("Ashrix Connector - Error").Error()
}

func ShowConfirmDialog(message string) bool {
    return dialog.Message("%s", message).Title("Ashrix Connector").YesNo()
}


func ShowTokenDialog() string {
	var code string

	var dlg *walk.Dialog
	var edit *walk.LineEdit
	var status *walk.Label

	dlg, _ = walk.NewDialog(nil)
	dlg.SetTitle("Verification Code")
	dlg.SetSize(walk.Size{Width: 300, Height: 150})

	label, _ := walk.NewLabel(dlg)
	label.SetText("Enter the 6-digit code:")
	label.SetBounds(walk.Rectangle{10, 10, 250, 20})

	edit, _ = walk.NewLineEdit(dlg)
	edit.SetBounds(walk.Rectangle{10, 35, 150, 24})

	status, _ = walk.NewLabel(dlg)
	status.SetBounds(walk.Rectangle{10, 65, 250, 20})

	ok, _ := walk.NewPushButton(dlg)
	ok.SetText("Verify")
	ok.SetBounds(walk.Rectangle{110, 95, 80, 28})

	ok.Clicked().Attach(func() {
		code = edit.Text()

		if !regexp.MustCompile(`^\d{6}$`).MatchString(code) {
			status.SetText("❌ Code must be exactly 6 digits.")
			return
		}

		// Example validation
		if code == "123456" {
			status.SetText("✅ Verification successful.")
		} else {
			status.SetText("❌ Invalid code.")
		}
	})

	cancel, _ := walk.NewPushButton(dlg)
	cancel.SetText("Cancel")
	cancel.SetBounds(walk.Rectangle{200, 95, 80, 28})
	cancel.Clicked().Attach(func() {
		dlg.Cancel()
	})

	dlg.Run()

	fmt.Println("Entered:", code)
	return code
}