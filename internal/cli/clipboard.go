package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// readClipboard reads the OS clipboard's text contents, shelling out to the
// platform-native tool since there's no cross-platform clipboard library in
// go.mod. Used by ".edit -c".
func readClipboard() (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbpaste")
	case "windows":
		cmd = exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard")
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to read clipboard (needs pbpaste/xclip/Get-Clipboard on PATH): %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}
