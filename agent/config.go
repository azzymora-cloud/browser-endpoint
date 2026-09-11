package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	serviceName = "ThinkCentreEndpoint"
	serviceDisp = "ThinkCentre Endpoint Agent"
	serviceDesc = "Manages the ThinkCentre browser remote stack, connection audit log, Wake-on-LAN, reboot, and session HUD kill switch."
	defaultPort = "18765"
)

type Config struct {
	Listen         string `json:"listen"`
	Token          string `json:"token"`
	StackDir       string `json:"stackDir"`
	RebootDelaySec int    `json:"rebootDelaySec"`
}

func defaultConfigPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("ProgramData"), "ThinkCentreEndpoint", "config.json")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".config", "thinkcentre-endpoint", "config.json")
	}
	return "agent.json"
}

func defaultStackDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("ProgramFiles"), "ThinkCentre Endpoint")
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "..", "docker-compose.yml")); err == nil {
			abs, _ := filepath.Abs(filepath.Join(dir, ".."))
			return abs
		}
	}
	wd, _ := os.Getwd()
	return wd
}

func loadOrCreateConfig(path string) (Config, error) {
	cfg := Config{
		Listen:         "127.0.0.1:" + defaultPort,
		StackDir:       defaultStackDir(),
		RebootDelaySec: 10,
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
		if cfg.Listen == "" {
			cfg.Listen = "127.0.0.1:" + defaultPort
		}
		if cfg.StackDir == "" {
			cfg.StackDir = defaultStackDir()
		}
		if cfg.RebootDelaySec <= 0 {
			cfg.RebootDelaySec = 10
		}
		return cfg, nil
	}
	if !os.IsNotExist(err) {
		return cfg, err
	}
	token, err := randomToken(24)
	if err != nil {
		return cfg, err
	}
	cfg.Token = token
	if err := saveConfig(path, cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func saveConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
