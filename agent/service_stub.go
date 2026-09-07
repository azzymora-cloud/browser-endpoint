//go:build !windows

package main

func isWindowsService() bool { return false }

func runWindowsService() error {
	return errString("Windows service mode is only available on Windows")
}

func installService() error {
	return errString("service install is only available on Windows")
}

func uninstallService() error {
	return errString("service uninstall is only available on Windows")
}
