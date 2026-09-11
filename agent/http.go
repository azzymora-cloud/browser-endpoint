package main

import (
	"crypto/subtle"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

type apiServer struct {
	cfg     Config
	started time.Time
	ui      fs.FS
	mon     *accessMonitor
}

func (s *apiServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/status", s.requireAuth(s.handleStatus))
	mux.HandleFunc("/api/reboot", s.requireAuth(s.handleReboot))
	mux.HandleFunc("/api/reboot/cancel", s.requireAuth(s.handleCancelReboot))
	mux.HandleFunc("/api/wake", s.requireAuth(s.handleWake))
	mux.HandleFunc("/api/wol/enable", s.requireAuth(s.handleEnableWOL))
	mux.HandleFunc("/api/stack/up", s.requireAuth(s.handleStackUp))
	mux.HandleFunc("/api/stack/down", s.requireAuth(s.handleStackDown))
	mux.HandleFunc("/api/connections", s.requireAuth(s.handleConnections))
	mux.HandleFunc("/api/hud/foreground", s.handleHudForeground)
	mux.HandleFunc("/api/hud/kill-foreground", s.handleHudKillForeground)
	mux.Handle("/", s.uiHandler())
	return withSecurity(mux)
}

func withSecurity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *apiServer) tokenOK(got string) bool {
	got = strings.TrimSpace(got)
	if s.cfg.Token == "" || got == "" {
		return false
	}
	if len(got) != len(s.cfg.Token) {
		// Still compare to keep timing flatter when lengths differ.
		subtle.ConstantTimeCompare([]byte(s.cfg.Token), []byte(s.cfg.Token))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.Token)) == 1
}

func (s *apiServer) requestToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if t := r.Header.Get("X-Agent-Token"); t != "" {
		return t
	}
	if c, err := r.Cookie("agent_token"); err == nil {
		return c.Value
	}
	return ""
}

func (s *apiServer) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.tokenOK(s.requestToken(r)) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *apiServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if !s.tokenOK(body.Token) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "wrong token"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "agent_token",
		Value:    s.cfg.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   12 * 60 * 60,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *apiServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	adapters, err := listAdapters()
	if err != nil {
		adapters = nil
	}
	payload := map[string]any{
		"hostname":  hostname(),
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
		"goos":      runtime.GOOS,
		"listen":    s.cfg.Listen,
		"stackDir":  s.cfg.StackDir,
		"uptime":    startedAgo(s.started),
		"time":      nowUTC(),
		"windows":   runtime.GOOS == "windows",
		"canReboot": runtime.GOOS == "windows",
		"wolNote":   wolNote(),
		"adapters":  adapters,
		"stack":     stackStatus(s.cfg),
		"rebootSec": s.cfg.RebootDelaySec,
	}
	if s.mon != nil {
		payload["connections"] = s.mon.Status()
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *apiServer) handleConnections(w http.ResponseWriter, r *http.Request) {
	if s.mon == nil {
		writeJSON(w, http.StatusOK, map[string]any{"events": []any{}, "path": ""})
		return
	}
	events, err := s.mon.Recent(80)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if events == nil {
		events = []ConnEvent{}
	}
	st := s.mon.Status()
	st["events"] = events
	writeJSON(w, http.StatusOK, st)
}

func wolNote() string {
	if runtime.GOOS != "windows" {
		return "This preview host is not the ThinkCentre. On Windows the agent enables magic-packet wake on the NIC; BIOS still has to allow PME/WoL."
	}
	return "The PC must be plugged into Ethernet. If it is fully powered off, send a magic packet from a phone on home Wi‑Fi or from another PC using thinkcentre-agent wake --mac. The agent cannot wake a machine that is already off — nothing is listening."
}

func (s *apiServer) handleReboot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	var body struct {
		Confirm string `json:"confirm"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !strings.EqualFold(body.Confirm, "reboot") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "confirm must be \"reboot\""})
		return
	}
	if err := scheduleReboot(s.cfg.RebootDelaySec); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Reboot scheduled",
		"seconds": s.cfg.RebootDelaySec,
	})
}

func (s *apiServer) handleCancelReboot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if err := cancelReboot(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *apiServer) handleWake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	var body struct {
		MAC       string `json:"mac"`
		Broadcast string `json:"broadcast"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	targets := []string{}
	if strings.TrimSpace(body.MAC) != "" {
		targets = []string{body.MAC}
	} else {
		adapters, err := listAdapters()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		for _, a := range adapters {
			if a.Up {
				targets = append(targets, a.MAC)
			}
		}
	}
	if len(targets) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "no MAC addresses to wake"})
		return
	}
	var errs []string
	sent := 0
	for _, raw := range targets {
		mac, err := parseMAC(raw)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if err := SendMagicPacket(mac, body.Broadcast); err != nil {
			errs = append(errs, mac.String()+": "+err.Error())
			continue
		}
		sent++
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      sent > 0,
		"sent":    sent,
		"targets": targets,
		"errors":  errs,
	})
}

func (s *apiServer) handleEnableWOL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	out, err := enableWakeOnLAN(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "output": out})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": out})
}

func (s *apiServer) handleStackUp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	out, err := stackUp(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "output": out})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": out})
}

func (s *apiServer) handleStackDown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	out, err := stackDown(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "output": out})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": out})
}

func (s *apiServer) handleHudForeground(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET required"})
		return
	}
	result := hudInspectFromService()
	writeJSON(w, http.StatusOK, result)
}

func (s *apiServer) handleHudKillForeground(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	var body struct {
		Confirm string `json:"confirm"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !strings.EqualFold(body.Confirm, "kill") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "confirm must be \"kill\""})
		return
	}
	result := hudKillFromService()
	status := http.StatusOK
	if !result.OK {
		status = http.StatusConflict
	}
	writeJSON(w, status, result)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *apiServer) uiHandler() http.Handler {
	if s.ui == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}
	fileServer := http.FileServer(http.FS(s.ui))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func serveAgent(cfg Config, ui fs.FS) error {
	mon := newAccessMonitor(cfg)
	go mon.Run()
	s := &apiServer{cfg: cfg, started: time.Now(), ui: ui, mon: mon}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return err
	}
	log.Printf("ThinkCentre Endpoint agent on http://%s (stack %s)", cfg.Listen, cfg.StackDir)
	log.Printf("Connection log: %s", mon.logPath)
	srv := &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 8 * time.Second,
	}
	return srv.Serve(ln)
}

func printTokenHint(cfg Config, path string) {
	if os.Getenv("THINKCENTRE_AGENT_QUIET") == "1" {
		return
	}
	log.Printf("Config: %s", path)
	log.Printf("Management token is stored in that file. Open http://%s and paste it.", cfg.Listen)
}
