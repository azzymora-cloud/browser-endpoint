package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type StackStatus struct {
	Dir        string         `json:"dir"`
	ComposeOK  bool           `json:"composeOk"`
	DockerBin  string         `json:"dockerBin"`
	Error      string         `json:"error,omitempty"`
	Services   []StackService `json:"services"`
	TunnelFile bool           `json:"tunnelFile"`
}

type StackService struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Health  string `json:"health,omitempty"`
	Status  string `json:"status"`
	Running bool   `json:"running"`
}

func findDocker() string {
	if p, err := exec.LookPath("docker"); err == nil {
		return p
	}
	if runtime.GOOS == "windows" {
		candidates := []string{
			filepath.Join(os.Getenv("ProgramFiles"), "Docker", "Docker", "resources", "bin", "docker.exe"),
			`C:\Program Files\Docker\Docker\resources\bin\docker.exe`,
		}
		for _, c := range candidates {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c
			}
		}
	}
	return ""
}

func composeArgs(cfg Config, extra ...string) []string {
	args := []string{"compose", "-f", "docker-compose.yml"}
	if tunnelOverlayEnabled(cfg) {
		args = append(args, "-f", "docker-compose.tunnel.yml")
	}
	return append(args, extra...)
}

func tunnelOverlayEnabled(cfg Config) bool {
	if _, err := os.Stat(filepath.Join(cfg.StackDir, "docker-compose.tunnel.yml")); err != nil {
		return false
	}
	if strings.TrimSpace(os.Getenv("CLOUDFLARE_TUNNEL_TOKEN")) != "" {
		return true
	}
	data, err := os.ReadFile(filepath.Join(cfg.StackDir, ".env"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "CLOUDFLARE_TUNNEL_TOKEN=") {
			val := strings.Trim(strings.TrimPrefix(line, "CLOUDFLARE_TUNNEL_TOKEN="), `"'`)
			return strings.TrimSpace(val) != ""
		}
	}
	return false
}

func runCompose(cfg Config, extra ...string) (string, error) {
	bin := findDocker()
	if bin == "" {
		return "", errDockerMissing
	}
	cmd := exec.Command(bin, composeArgs(cfg, extra...)...)
	cmd.Dir = cfg.StackDir
	cmd.Env = os.Environ()
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

var errDockerMissing = errString("docker is not installed or not on PATH")

type errString string

func (e errString) Error() string { return string(e) }

func stackStatus(cfg Config) StackStatus {
	st := StackStatus{Dir: cfg.StackDir}
	if _, err := os.Stat(filepath.Join(cfg.StackDir, "docker-compose.yml")); err != nil {
		st.Error = "docker-compose.yml not found in " + cfg.StackDir
		return st
	}
	_, err := os.Stat(filepath.Join(cfg.StackDir, "docker-compose.tunnel.yml"))
	st.TunnelFile = err == nil
	st.DockerBin = findDocker()
	if st.DockerBin == "" {
		st.Error = errDockerMissing.Error()
		return st
	}
	out, err := runCompose(cfg, "ps", "--format", "json")
	if err != nil {
		st.Error = strings.TrimSpace(out)
		if st.Error == "" {
			st.Error = err.Error()
		}
		return st
	}
	st.ComposeOK = true
	trimmed := strings.TrimSpace(out)
	if strings.HasPrefix(trimmed, "[") {
		var rows []map[string]any
		if err := json.Unmarshal([]byte(trimmed), &rows); err == nil {
			for _, row := range rows {
				st.Services = append(st.Services, rowToService(row))
			}
			return st
		}
	}
	dec := json.NewDecoder(strings.NewReader(out))
	for dec.More() {
		var row map[string]any
		if err := dec.Decode(&row); err != nil {
			break
		}
		st.Services = append(st.Services, rowToService(row))
	}
	return st
}

func rowToService(row map[string]any) StackService {
	name := stringField(row, "Service")
	if name == "" {
		name = stringField(row, "Name")
	}
	state := stringField(row, "State")
	return StackService{
		Name:    name,
		State:   state,
		Health:  stringField(row, "Health"),
		Status:  stringField(row, "Status"),
		Running: strings.EqualFold(state, "running"),
	}
}

func stringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func stackUp(cfg Config) (string, error) {
	return runCompose(cfg, "up", "-d")
}

func stackDown(cfg Config) (string, error) {
	return runCompose(cfg, "stop")
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func startedAgo(since time.Time) string {
	d := time.Since(since).Round(time.Second)
	return d.String()
}
