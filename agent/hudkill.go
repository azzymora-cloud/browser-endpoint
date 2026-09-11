package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
)

type hudTarget struct {
	OK    bool   `json:"ok"`
	PID   uint32 `json:"pid,omitempty"`
	Name  string `json:"name,omitempty"`
	Title string `json:"title,omitempty"`
	Error string `json:"error,omitempty"`
}

func imageBase(name string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	return strings.ToLower(path.Base(name))
}

func protectedImage(name string) bool {
	switch imageBase(name) {
	case "", "system", "registry", "idle", "secure system", "memory compression",
		"smss.exe", "csrss.exe", "wininit.exe", "winlogon.exe", "services.exe",
		"lsass.exe", "lsaiso.exe", "dwm.exe", "explorer.exe", "fontdrvhost.exe",
		"sihost.exe", "taskhostw.exe", "searchhost.exe", "startmenuexperiencehost.exe",
		"shellexperiencehost.exe", "textinputhost.exe", "conhost.exe",
		"thinkcentre-agent.exe", "svchost.exe":
		return true
	default:
		return false
	}
}

func writeHudResult(outPath string, result hudTarget) error {
	if strings.TrimSpace(outPath) == "" {
		enc := json.NewEncoder(os.Stdout)
		return enc.Encode(result)
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, data, 0o600)
}

func runHudKill(args []string) error {
	outPath := ""
	kill := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-out", "--out":
			i++
			if i >= len(args) {
				return fmt.Errorf("hud-kill: -out requires a path")
			}
			outPath = args[i]
		case "-kill", "--kill":
			kill = true
		default:
			return fmt.Errorf("hud-kill: unknown argument %q", args[i])
		}
	}
	result := inspectForeground()
	if kill && result.OK {
		result = killForegroundPID(result)
	}
	if err := writeHudResult(outPath, result); err != nil {
		return err
	}
	return nil
}
