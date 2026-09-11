//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32                   = windows.NewLazySystemDLL("user32.dll")
	modWtsapi32                 = windows.NewLazySystemDLL("wtsapi32.dll")
	procGetForegroundWindow     = modUser32.NewProc("GetForegroundWindow")
	procGetWindowTextW          = modUser32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId = modUser32.NewProc("GetWindowThreadProcessId")
	procWTSQueryUserToken       = modWtsapi32.NewProc("WTSQueryUserToken")
)

func inspectForeground() hudTarget {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return hudTarget{Error: "no window is in focus"}
	}
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return hudTarget{Error: "could not read the focused process"}
	}
	name, err := processImage(pid)
	if err != nil {
		return hudTarget{Error: err.Error()}
	}
	if protectedImage(name) {
		return hudTarget{
			PID:   pid,
			Name:  filepath.Base(name),
			Title: windowTitle(hwnd),
			Error: "refusing to kill Windows shell process " + filepath.Base(name),
		}
	}
	if pid == uint32(os.Getpid()) {
		return hudTarget{Error: "refusing to kill the endpoint agent"}
	}
	return hudTarget{
		OK:    true,
		PID:   pid,
		Name:  filepath.Base(name),
		Title: windowTitle(hwnd),
	}
}

func killForegroundPID(target hudTarget) hudTarget {
	if !target.OK || target.PID == 0 {
		if target.Error == "" {
			target.Error = "nothing to kill"
		}
		target.OK = false
		return target
	}
	cmd := exec.Command("taskkill", "/PID", strconv.FormatUint(uint64(target.PID), 10), "/F")
	out, err := cmd.CombinedOutput()
	if err != nil {
		target.OK = false
		msg := string(out)
		if msg == "" {
			msg = err.Error()
		}
		target.Error = "taskkill: " + clip(msg, 180)
		return target
	}
	target.Error = ""
	return target
}

func processImage(pid uint32) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", fmt.Errorf("open process: %w", err)
	}
	defer windows.CloseHandle(h)
	var n uint32 = 32768
	buf := make([]uint16, n)
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return "", fmt.Errorf("process name: %w", err)
	}
	return windows.UTF16ToString(buf[:n]), nil
}

func windowTitle(hwnd uintptr) string {
	buf := make([]uint16, 512)
	n, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

func hudInspectFromService() hudTarget {
	if !isWindowsService() {
		return inspectForeground()
	}
	return runHudHelper(false)
}

func hudKillFromService() hudTarget {
	if !isWindowsService() {
		t := inspectForeground()
		if !t.OK {
			return t
		}
		return killForegroundPID(t)
	}
	return runHudHelper(true)
}

func runHudHelper(kill bool) hudTarget {
	exe, err := os.Executable()
	if err != nil {
		return hudTarget{Error: err.Error()}
	}

	token, err := tokenForActiveSession()
	if err != nil {
		return hudTarget{Error: err.Error()}
	}
	defer token.Close()

	args := []string{"hud-kill"}
	if kill {
		args = append(args, "-kill")
	}
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
		Token:      syscall.Token(token),
	}
	out, runErr := cmd.Output()
	if len(out) > 0 {
		var result hudTarget
		if err := json.Unmarshal(out, &result); err != nil {
			return hudTarget{Error: "helper output: " + err.Error()}
		}
		return result
	}
	if runErr != nil {
		return hudTarget{Error: "could not inspect the interactive desktop: " + runErr.Error()}
	}
	return hudTarget{Error: "helper produced no result"}
}

func tokenForActiveSession() (windows.Token, error) {
	sessionID, err := activeSessionID()
	if err != nil {
		return 0, err
	}
	var handle windows.Handle
	r1, _, callErr := procWTSQueryUserToken.Call(uintptr(sessionID), uintptr(unsafe.Pointer(&handle)))
	if r1 == 0 {
		return 0, fmt.Errorf("open interactive session token: %w", callErr)
	}
	return windows.Token(handle), nil
}

func activeSessionID() (uint32, error) {
	var info *windows.WTS_SESSION_INFO
	var count uint32
	err := windows.WTSEnumerateSessions(0, 0, 1, &info, &count)
	if err == nil && info != nil && count > 0 {
		defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(info)))
		sessions := unsafe.Slice(info, count)
		for _, s := range sessions {
			if s.SessionID != 0 && s.State == windows.WTSActive {
				return s.SessionID, nil
			}
		}
	}
	id := windows.WTSGetActiveConsoleSessionId()
	if id == 0xFFFFFFFF {
		return 0, fmt.Errorf("no active Windows session")
	}
	return id, nil
}
