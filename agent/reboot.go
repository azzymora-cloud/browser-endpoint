package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

func scheduleReboot(delaySec int) error {
	if delaySec < 3 {
		delaySec = 3
	}
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("shutdown", "/r", "/t", fmt.Sprintf("%d", delaySec), "/c", "ThinkCentre Endpoint requested a reboot")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("shutdown: %w (%s)", err, string(out))
		}
		return nil
	default:
		// Preview/dev hosts: do not actually reboot the machine.
		return fmt.Errorf("reboot is only issued on Windows (delay would have been %s)", time.Duration(delaySec)*time.Second)
	}
}

func cancelReboot() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	cmd := exec.Command("shutdown", "/a")
	_, _ = cmd.CombinedOutput()
	return nil
}
