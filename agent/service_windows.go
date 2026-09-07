//go:build windows

package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

func isWindowsService() bool {
	ok, err := svc.IsWindowsService()
	return err == nil && ok
}

type agentService struct{}

func (m *agentService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}
	cfg, err := loadOrCreateConfig(defaultConfigPath())
	if err != nil {
		log.Print(err)
		return true, 1
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
			cfg.StackDir = dir
			_ = saveConfig(defaultConfigPath(), cfg)
		}
	}
	ui, err := fs.Sub(embeddedWeb, "web")
	if err != nil {
		log.Print(err)
		return true, 1
	}
	go func() {
		if err := serveAgent(cfg, ui); err != nil {
			log.Print(err)
		}
	}()
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			return false, 0
		}
	}
	return false, 0
}

func runWindowsService() error {
	elog, err := eventlog.Open(serviceName)
	if err == nil {
		defer elog.Close()
		elog.Info(1, "ThinkCentre Endpoint agent starting")
	}
	return svc.Run(serviceName, &agentService{})
}

func installService() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s is already installed", serviceName)
	}
	s, err = m.CreateService(serviceName, exe, mgr.Config{
		DisplayName:  serviceDisp,
		Description:  serviceDesc,
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
	})
	if err != nil {
		return err
	}
	defer s.Close()
	if err := eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info); err != nil {
		_ = s.Delete()
		return err
	}
	cfg, err := loadOrCreateConfig(defaultConfigPath())
	if err != nil {
		return err
	}
	if dir := filepath.Dir(exe); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
			cfg.StackDir = dir
			_ = saveConfig(defaultConfigPath(), cfg)
		}
	}
	if err := s.Start(); err != nil {
		return fmt.Errorf("service installed but failed to start: %w", err)
	}
	fmt.Printf("Installed and started %s\n", serviceName)
	fmt.Printf("Management UI: http://%s\n", cfg.Listen)
	fmt.Printf("Token file:    %s\n", defaultConfigPath())
	time.Sleep(200 * time.Millisecond)
	return nil
}

func uninstallService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service %s is not installed", serviceName)
	}
	defer s.Close()
	_, _ = s.Control(svc.Stop)
	_ = s.Delete()
	_ = eventlog.Remove(serviceName)
	fmt.Printf("Removed %s\n", serviceName)
	return nil
}
