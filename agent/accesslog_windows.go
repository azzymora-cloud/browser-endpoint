//go:build windows

package main

import (
	"os/exec"
	"strings"
)

func readRDPLogons(afterRecord int64) ([]rdpEvent, error) {
	cmd := exec.Command("wevtutil", "qe", "Security",
		`/q:*[System[(EventID=4624 or EventID=4625)]]`,
		"/f:xml",
		"/c:40",
		"/rd:true",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out) + " " + err.Error())
		low := strings.ToLower(msg)
		if strings.Contains(low, "access is denied") || strings.Contains(low, "cannot open") {
			return nil, nil
		}
		return nil, errString("wevtutil: " + clip(msg, 200))
	}
	events := parseSecurityEvents(string(out))
	if afterRecord <= 0 {
		return events, nil
	}
	var fresh []rdpEvent
	for _, ev := range events {
		if ev.record > afterRecord {
			fresh = append(fresh, ev)
		}
	}
	return fresh, nil
}
