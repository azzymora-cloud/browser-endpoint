package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"
)

//go:embed web/*
var embeddedWeb embed.FS

func main() {
	log.SetFlags(log.LstdFlags)
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return runServiceOrForeground("")
	}
	switch args[0] {
	case "run", "serve":
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		cfgPath := fs.String("config", defaultConfigPath(), "config json path")
		listen := fs.String("listen", "", "override listen address")
		stack := fs.String("stack", "", "override stack directory")
		_ = fs.Parse(args[1:])
		return runForeground(*cfgPath, *listen, *stack)
	case "install":
		return installService()
	case "uninstall":
		return uninstallService()
	case "wake":
		fs := flag.NewFlagSet("wake", flag.ExitOnError)
		mac := fs.String("mac", "", "target MAC (required)")
		bcast := fs.String("bcast", "255.255.255.255", "IPv4 broadcast address")
		_ = fs.Parse(args[1:])
		if strings.TrimSpace(*mac) == "" {
			return fmt.Errorf("usage: thinkcentre-agent wake --mac AA:BB:CC:DD:EE:FF [--bcast 192.168.1.255]")
		}
		hw, err := parseMAC(*mac)
		if err != nil {
			return err
		}
		if err := SendMagicPacket(hw, *bcast); err != nil {
			return err
		}
		fmt.Printf("Sent magic packet to %s via %s\n", strings.ToUpper(hw.String()), *bcast)
		return nil
	case "enable-wol":
		cfg, err := loadOrCreateConfig(defaultConfigPath())
		if err != nil {
			return err
		}
		out, err := enableWakeOnLAN(cfg)
		fmt.Print(out)
		return err
	case "version":
		fmt.Println("thinkcentre-endpoint-agent 1.2.0")
		return nil
	default:
		return fmt.Errorf("unknown command %q (run|install|uninstall|wake|enable-wol|version)", args[0])
	}
}

func runServiceOrForeground(cfgPath string) error {
	if isWindowsService() {
		return runWindowsService()
	}
	if cfgPath == "" {
		cfgPath = defaultConfigPath()
	}
	return runForeground(cfgPath, "", "")
}

func runForeground(cfgPath, listen, stack string) error {
	cfg, err := loadOrCreateConfig(cfgPath)
	if err != nil {
		return err
	}
	if listen != "" {
		cfg.Listen = listen
	}
	if stack != "" {
		cfg.StackDir = stack
	}
	printTokenHint(cfg, cfgPath)
	ui, err := fs.Sub(embeddedWeb, "web")
	if err != nil {
		return err
	}
	return serveAgent(cfg, ui)
}
