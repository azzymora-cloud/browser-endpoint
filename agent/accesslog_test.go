package main

import (
	"strings"
	"testing"
)

func TestParseGuacConnRows(t *testing.T) {
	raw := "2|guacadmin|172.18.0.1|ThinkCentre (RDP)|2026-09-10 12:00:00+00|2026-09-10 12:05:00+00\n1|guacadmin|172.18.0.1|ThinkCentre (RDP)|2026-09-10 11:00:00+00|\n"
	rows := parseGuacConnRows(raw)
	if len(rows) != 2 {
		t.Fatalf("len=%d", len(rows))
	}
	if rows[0].ID != 1 || rows[1].ID != 2 {
		t.Fatalf("order %+v", rows)
	}
	if rows[1].Connection != "ThinkCentre (RDP)" || rows[1].User != "guacadmin" {
		t.Fatalf("%+v", rows[1])
	}
}

func TestParseGuacLoginRows(t *testing.T) {
	raw := "3|guacadmin|10.0.0.8|2026-09-10 12:00:00+00|\n"
	rows := parseGuacLoginRows(raw)
	if len(rows) != 1 || rows[0].IP != "10.0.0.8" {
		t.Fatalf("%+v", rows)
	}
}

func TestParseCloudflaredLine(t *testing.T) {
	ev, ok := parseCloudflaredLine(`INF GET https://wintermute.boxbox.trade/ status=200`)
	if !ok || ev.Result != "success" {
		t.Fatalf("%v %+v", ok, ev)
	}
	ev, ok = parseCloudflaredLine(`ERR failed to serve incoming request`)
	if !ok || ev.Result != "fail" {
		t.Fatalf("err line %v %+v", ok, ev)
	}
	if _, ok := parseCloudflaredLine(`INF Registered tunnel connection`); ok {
		t.Fatal("noise should be ignored")
	}
}

func TestParseSecurityEvents(t *testing.T) {
	raw := `<Event xmlns="http://schemas.microsoft.com/win/2004/08/events/event"><System><EventID>4624</EventID><EventRecordID>99</EventRecordID><TimeCreated SystemTime="2026-09-10T16:01:02.000000000Z"/></System><EventData><Data Name="TargetUserName">primus</Data><Data Name="IpAddress">172.22.32.1</Data><Data Name="LogonType">10</Data></EventData></Event>`
	got := parseSecurityEvents(raw)
	if len(got) != 1 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].User != "primus" || got[0].IP != "172.22.32.1" || got[0].Result != "success" {
		t.Fatalf("%+v", got[0])
	}
	if got[0].Connection != "ThinkCentre (RDP)" {
		t.Fatalf("conn %s", got[0].Connection)
	}
}

func TestSafeSQLIdent(t *testing.T) {
	if !safeSQLIdent("guacamole_db") || safeSQLIdent("guac;drop") || safeSQLIdent("") {
		t.Fatal("ident filter")
	}
}

func TestClip(t *testing.T) {
	if !strings.HasSuffix(clip("abcdefghij", 4), "…") {
		t.Fatal(clip("abcdefghij", 4))
	}
}
