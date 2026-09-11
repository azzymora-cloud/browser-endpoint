package main

import "testing"

func TestProtectedImage(t *testing.T) {
	protected := []string{
		"explorer.exe",
		`C:\Windows\explorer.exe`,
		"C:/Windows/System32/csrss.exe",
		"DWM.EXE",
		"thinkcentre-agent.exe",
		"svchost.exe",
	}
	for _, name := range protected {
		if !protectedImage(name) {
			t.Fatalf("expected protected %q", name)
		}
	}
	open := []string{"notepad.exe", `C:\Program Files\App\frozen.exe`, "chrome.exe"}
	for _, name := range open {
		if protectedImage(name) {
			t.Fatalf("expected killable %q", name)
		}
	}
}

func TestImageBase(t *testing.T) {
	if got := imageBase(`C:\Windows\System32\notepad.exe`); got != "notepad.exe" {
		t.Fatalf("got %q", got)
	}
}
