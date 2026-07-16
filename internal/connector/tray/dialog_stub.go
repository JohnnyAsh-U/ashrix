//go:build !windows

package tray

func ShowTokenDialog() string {
    return ""
}

func ShowInfoDialog(message string) {}

func ShowErrorDialog(message string) {}

func ShowConfirmDialog(message string) bool {
    return false
}