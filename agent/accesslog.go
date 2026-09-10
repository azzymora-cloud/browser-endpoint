package main

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	connLogFileName = "connections.jsonl"
	connCursorName  = "connections.cursor.json"
	maxConnLogBytes = 8 << 20
	connPollEvery   = 15 * time.Second
)

type ConnEvent struct {
	TS         string `json:"ts"`
	Source     string `json:"source"`
	User       string `json:"user,omitempty"`
	IP         string `json:"ip,omitempty"`
	Connection string `json:"connection,omitempty"`
	Result     string `json:"result"`
	Detail     string `json:"detail,omitempty"`
	ID         string `json:"id"`
}

type monitorCursor struct {
	GuacHistoryID int64             `json:"guacHistoryId"`
	GuacLoginID   int64             `json:"guacLoginId"`
	OpenGuac      map[string]string `json:"openGuac,omitempty"`
	OpenLogins    map[string]string `json:"openLogins,omitempty"`
	LastRDPRecord int64             `json:"lastRdpRecord"`
	CFSeen        map[string]int64  `json:"cfSeen,omitempty"`
}

type accessMonitor struct {
	cfg        Config
	dir        string
	logPath    string
	cursorPath string

	mu     sync.Mutex
	cursor monitorCursor
	err    string
	last   time.Time
}

func defaultLogDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("ProgramData"), "ThinkCentreEndpoint", "logs")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".config", "thinkcentre-endpoint", "logs")
	}
	return "logs"
}

func newAccessMonitor(cfg Config) *accessMonitor {
	dir := defaultLogDir()
	m := &accessMonitor{
		cfg:        cfg,
		dir:        dir,
		logPath:    filepath.Join(dir, connLogFileName),
		cursorPath: filepath.Join(dir, connCursorName),
	}
	m.cursor.OpenGuac = map[string]string{}
	m.cursor.OpenLogins = map[string]string{}
	m.cursor.CFSeen = map[string]int64{}
	_ = m.loadCursor()
	return m
}

func (m *accessMonitor) Run() {
	m.append(ConnEvent{
		TS:     nowUTC(),
		Source: "agent",
		Result: "ok",
		Detail: "connection monitor started",
		ID:     "agent-start-" + strconv.FormatInt(time.Now().Unix(), 10),
	})
	m.poll()
	t := time.NewTicker(connPollEvery)
	defer t.Stop()
	for range t.C {
		m.poll()
	}
}

func (m *accessMonitor) poll() {
	m.mu.Lock()
	m.last = time.Now()
	m.mu.Unlock()

	var errs []string
	if err := m.pollGuacamole(); err != nil {
		errs = append(errs, "guacamole: "+err.Error())
	}
	if err := m.pollCloudflared(); err != nil {
		errs = append(errs, "cloudflared: "+err.Error())
	}
	if err := m.pollRDP(); err != nil {
		errs = append(errs, "rdp: "+err.Error())
	}
	m.mu.Lock()
	if len(errs) == 0 {
		m.err = ""
	} else {
		m.err = strings.Join(errs, "; ")
	}
	m.mu.Unlock()
	_ = m.saveCursor()
}

func (m *accessMonitor) Status() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := map[string]any{
		"path":     m.logPath,
		"lastPoll": m.last.UTC().Format(time.RFC3339),
	}
	if m.err != "" {
		st["error"] = m.err
	}
	return st
}

func (m *accessMonitor) Recent(limit int) ([]ConnEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 80
	}
	f, err := os.Open(m.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) > limit*4 {
			lines = lines[len(lines)-limit:]
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	out := make([]ConnEvent, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		var ev ConnEvent
		if err := json.Unmarshal([]byte(lines[i]), &ev); err != nil {
			continue
		}
		out = append(out, ev)
	}
	return out, nil
}

func (m *accessMonitor) loadCursor() error {
	data, err := os.ReadFile(m.cursorPath)
	if err != nil {
		return err
	}
	var c monitorCursor
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	if c.OpenGuac == nil {
		c.OpenGuac = map[string]string{}
	}
	if c.OpenLogins == nil {
		c.OpenLogins = map[string]string{}
	}
	if c.CFSeen == nil {
		c.CFSeen = map[string]int64{}
	}
	m.cursor = c
	return nil
}

func (m *accessMonitor) saveCursor() error {
	m.mu.Lock()
	c := m.cursor
	m.mu.Unlock()
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.cursorPath, data, 0o600)
}

func (m *accessMonitor) append(ev ConnEvent) {
	if ev.TS == "" {
		ev.TS = nowUTC()
	}
	if ev.ID == "" {
		ev.ID = ev.Source + "-" + ev.TS
	}
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return
	}
	if st, err := os.Stat(m.logPath); err == nil && st.Size() > maxConnLogBytes {
		_ = os.Rename(m.logPath, m.logPath+".1")
	}
	f, err := os.OpenFile(m.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = f.Write(append(data, '\n'))
	_ = f.Close()
}

func postgresIdent(cfg Config) (user, db string) {
	user, db = "guacamole", "guacamole_db"
	data, err := os.ReadFile(filepath.Join(cfg.StackDir, ".env"))
	if err != nil {
		return user, db
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if i := strings.IndexByte(line, '='); i > 0 {
			k := strings.TrimSpace(line[:i])
			v := strings.Trim(strings.TrimSpace(line[i+1:]), `"'`)
			switch k {
			case "POSTGRES_USER":
				if v != "" {
					user = v
				}
			case "POSTGRES_DB":
				if v != "" {
					db = v
				}
			}
		}
	}
	return user, db
}

func (m *accessMonitor) pollGuacamole() error {
	user, db := postgresIdent(m.cfg)
	if !safeSQLIdent(user) || !safeSQLIdent(db) {
		return errString("unsafe postgres identifiers")
	}
	connSQL := `SELECT history_id, username, COALESCE(remote_host,''), connection_name, start_date, COALESCE(end_date::text,'') FROM guacamole_connection_history ORDER BY history_id DESC LIMIT 80`
	loginSQL := `SELECT history_id, username, COALESCE(remote_host,''), start_date, COALESCE(end_date::text,'') FROM guacamole_user_history ORDER BY history_id DESC LIMIT 80`
	connOut, err := m.psql(user, db, connSQL)
	if err != nil {
		return err
	}
	loginOut, err := m.psql(user, db, loginSQL)
	if err != nil {
		return err
	}
	m.applyGuacConnections(parseGuacConnRows(connOut))
	m.applyGuacLogins(parseGuacLoginRows(loginOut))
	return nil
}

func (m *accessMonitor) psql(user, db, sql string) (string, error) {
	if id := dockerContainerID("postgres"); id != "" {
		return runDocker("exec", "-i", id, "psql", "-U", user, "-d", db, "-At", "-F", "|", "-c", sql)
	}
	return runCompose(m.cfg, "exec", "-T", "postgres", "psql", "-U", user, "-d", db, "-At", "-F", "|", "-c", sql)
}

func safeSQLIdent(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

type guacConnRow struct {
	ID         int64
	User       string
	IP         string
	Connection string
	Start      string
	End        string
}

type guacLoginRow struct {
	ID    int64
	User  string
	IP    string
	Start string
	End   string
}

func parseGuacConnRows(out string) []guacConnRow {
	var rows []guacConnRow
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "ERROR") {
			continue
		}
		p := strings.Split(line, "|")
		if len(p) < 6 {
			continue
		}
		id, err := strconv.ParseInt(p[0], 10, 64)
		if err != nil {
			continue
		}
		rows = append(rows, guacConnRow{
			ID:         id,
			User:       p[1],
			IP:         p[2],
			Connection: p[3],
			Start:      p[4],
			End:        p[5],
		})
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows
}

func parseGuacLoginRows(out string) []guacLoginRow {
	var rows []guacLoginRow
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "ERROR") {
			continue
		}
		p := strings.Split(line, "|")
		if len(p) < 5 {
			continue
		}
		id, err := strconv.ParseInt(p[0], 10, 64)
		if err != nil {
			continue
		}
		rows = append(rows, guacLoginRow{ID: id, User: p[1], IP: p[2], Start: p[3], End: p[4]})
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows
}

func (m *accessMonitor) applyGuacConnections(rows []guacConnRow) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cursor.OpenGuac == nil {
		m.cursor.OpenGuac = map[string]string{}
	}
	for _, r := range rows {
		key := strconv.FormatInt(r.ID, 10)
		if r.ID > m.cursor.GuacHistoryID {
			m.appendLocked(ConnEvent{
				TS:         rfcOrRaw(r.Start),
				Source:     "guacamole",
				User:       r.User,
				IP:         r.IP,
				Connection: r.Connection,
				Result:     "success",
				Detail:     "session started",
				ID:         "guac-start-" + key,
			})
			m.cursor.GuacHistoryID = r.ID
			if r.End == "" {
				m.cursor.OpenGuac[key] = r.Connection
			}
		}
		if r.End != "" {
			if _, open := m.cursor.OpenGuac[key]; open || r.ID == m.cursor.GuacHistoryID {
				if _, open := m.cursor.OpenGuac[key]; open {
					m.appendLocked(ConnEvent{
						TS:         rfcOrRaw(r.End),
						Source:     "guacamole",
						User:       r.User,
						IP:         r.IP,
						Connection: r.Connection,
						Result:     "end",
						Detail:     "session ended",
						ID:         "guac-end-" + key,
					})
					delete(m.cursor.OpenGuac, key)
				}
			}
		} else if r.ID <= m.cursor.GuacHistoryID {
			m.cursor.OpenGuac[key] = r.Connection
		}
	}
}

func (m *accessMonitor) applyGuacLogins(rows []guacLoginRow) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cursor.OpenLogins == nil {
		m.cursor.OpenLogins = map[string]string{}
	}
	for _, r := range rows {
		key := strconv.FormatInt(r.ID, 10)
		if r.ID > m.cursor.GuacLoginID {
			m.appendLocked(ConnEvent{
				TS:     rfcOrRaw(r.Start),
				Source: "guacamole-login",
				User:   r.User,
				IP:     r.IP,
				Result: "success",
				Detail: "Guacamole web login",
				ID:     "guac-login-" + key,
			})
			m.cursor.GuacLoginID = r.ID
			if r.End == "" {
				m.cursor.OpenLogins[key] = r.User
			}
		}
		if r.End != "" {
			if _, open := m.cursor.OpenLogins[key]; open {
				m.appendLocked(ConnEvent{
					TS:     rfcOrRaw(r.End),
					Source: "guacamole-login",
					User:   r.User,
					IP:     r.IP,
					Result: "end",
					Detail: "Guacamole web logout",
					ID:     "guac-logout-" + key,
				})
				delete(m.cursor.OpenLogins, key)
			}
		}
	}
}

func (m *accessMonitor) appendLocked(ev ConnEvent) {
	// caller holds m.mu; release around disk write to keep polls short
	m.mu.Unlock()
	m.append(ev)
	m.mu.Lock()
}

func rfcOrRaw(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nowUTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	layouts := []string{
		"2006-01-02 15:04:05.999999-07",
		"2006-01-02 15:04:05.999999+00",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05+00",
		time.RFC3339Nano,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return s
}

var cfReqRe = regexp.MustCompile(`(?i)\b(GET|POST|PUT|HEAD|OPTIONS)\b.*\b(status|statusCode)=(\d+)`)
var cfHostRe = regexp.MustCompile(`(?i)(https?://[^\s]+)`)
var cfIPRe = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

func parseCloudflaredLine(line string) (ConnEvent, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return ConnEvent{}, false
	}
	low := strings.ToLower(line)
	interesting := strings.Contains(low, " err ") || strings.HasPrefix(low, "err") ||
		strings.Contains(low, "incoming request") || cfReqRe.MatchString(line) ||
		(strings.Contains(low, "get ") && strings.Contains(low, "status"))
	if !interesting {
		return ConnEvent{}, false
	}
	ev := ConnEvent{
		TS:     nowUTC(),
		Source: "cloudflared",
		Result: "success",
		Detail: clip(line, 240),
	}
	if strings.Contains(low, "err") || strings.Contains(low, "failed") || strings.Contains(low, "error") {
		ev.Result = "fail"
	}
	if m := cfHostRe.FindString(line); m != "" {
		ev.Connection = m
	}
	if m := cfIPRe.FindString(line); m != "" && !strings.HasPrefix(m, "127.") {
		ev.IP = m
	}
	return ev, true
}

func (m *accessMonitor) pollCloudflared() error {
	var out string
	var err error
	if id := dockerContainerID("cloudflared"); id != "" {
		out, err = runDocker("logs", "--timestamps", "--since", "2m", id)
	} else {
		out, err = runCompose(m.cfg, "logs", "--no-color", "--since", "2m", "cloudflared")
	}
	if err != nil {
		// Tunnel overlay may be down; not fatal for the rest of the monitor.
		msg := strings.ToLower(out + " " + err.Error())
		if strings.Contains(msg, "no such service") || strings.Contains(msg, "no such container") {
			return nil
		}
		return fmt.Errorf("%s", strings.TrimSpace(out+" "+err.Error()))
	}
	now := time.Now().Unix()
	m.mu.Lock()
	if m.cursor.CFSeen == nil {
		m.cursor.CFSeen = map[string]int64{}
	}
	for k, ts := range m.cursor.CFSeen {
		if now-ts > 3600 {
			delete(m.cursor.CFSeen, k)
		}
	}
	m.mu.Unlock()
	for _, line := range strings.Split(out, "\n") {
		ev, ok := parseCloudflaredLine(line)
		if !ok {
			continue
		}
		sum := ev.Detail
		m.mu.Lock()
		if _, seen := m.cursor.CFSeen[sum]; seen {
			m.mu.Unlock()
			continue
		}
		m.cursor.CFSeen[sum] = now
		m.mu.Unlock()
		ev.ID = "cf-" + strconv.FormatInt(now, 10) + "-" + strconv.Itoa(len(sum))
		m.append(ev)
	}
	return nil
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func (m *accessMonitor) pollRDP() error {
	events, err := readRDPLogons(m.cursor.LastRDPRecord)
	if err != nil {
		return err
	}
	m.mu.Lock()
	last := m.cursor.LastRDPRecord
	m.mu.Unlock()
	for _, ev := range events {
		rec := ev.record
		if rec <= last {
			continue
		}
		m.append(ev.ConnEvent)
		if rec > last {
			last = rec
		}
	}
	m.mu.Lock()
	m.cursor.LastRDPRecord = last
	m.mu.Unlock()
	return nil
}

type rdpEvent struct {
	ConnEvent
	record int64
}

type wevtEvent struct {
	System struct {
		EventID       string `xml:"EventID"`
		EventRecordID int64  `xml:"EventRecordID"`
		TimeCreated   struct {
			SystemTime string `xml:"SystemTime,attr"`
		} `xml:"TimeCreated"`
	} `xml:"System"`
	EventData struct {
		Data []struct {
			Name  string `xml:"Name,attr"`
			Value string `xml:",chardata"`
		} `xml:"Data"`
	} `xml:"EventData"`
}

func (e wevtEvent) data(name string) string {
	for _, d := range e.EventData.Data {
		if d.Name == name {
			return strings.TrimSpace(d.Value)
		}
	}
	return ""
}

func parseSecurityEvents(raw string) []rdpEvent {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	chunks := strings.Split(raw, "<Event ")
	var out []rdpEvent
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		if !strings.HasPrefix(chunk, "<Event") {
			chunk = "<Event " + chunk
		}
		var ev wevtEvent
		if err := xml.Unmarshal([]byte(chunk), &ev); err != nil {
			continue
		}
		logonType := ev.data("LogonType")
		if logonType != "10" && logonType != "3" {
			continue
		}
		user := ev.data("TargetUserName")
		if user == "" || strings.HasSuffix(user, "$") {
			continue
		}
		ip := ev.data("IpAddress")
		if ip == "-" {
			ip = ""
		}
		if logonType == "3" {
			proc := strings.ToLower(ev.data("LogonProcessName") + " " + ev.data("AuthenticationPackageName"))
			if !strings.Contains(proc, "ntlmssp") && !strings.Contains(strings.ToLower(ev.data("WorkstationName")), "rdp") {
				continue
			}
		}
		result := "success"
		detail := "RDP logon type " + logonType
		if ev.System.EventID == "4625" {
			result = "fail"
			detail = "RDP logon failed (type " + logonType + ")"
		}
		out = append(out, rdpEvent{
			record: ev.System.EventRecordID,
			ConnEvent: ConnEvent{
				TS:         rfcOrRaw(ev.System.TimeCreated.SystemTime),
				Source:     "rdp",
				User:       user,
				IP:         ip,
				Connection: "ThinkCentre (RDP)",
				Result:     result,
				Detail:     detail,
				ID:         "rdp-" + strconv.FormatInt(ev.System.EventRecordID, 10),
			},
		})
	}
	return out
}
