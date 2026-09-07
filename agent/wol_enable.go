package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func enableWakeOnLAN(cfg Config) (string, error) {
	script := filepath.Join(cfg.StackDir, "scripts", "enable-wol.ps1")
	if runtime.GOOS != "windows" {
		return "Wake-on-LAN NIC flags are configured only on Windows. Magic-packet send still works from this host.", nil
	}
	if _, err := os.Stat(script); err != nil {
		return "", fmt.Errorf("missing %s", script)
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("enable-wol: %w", err)
	}
	return string(out), nil
}
