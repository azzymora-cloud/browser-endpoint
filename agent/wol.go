package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

// MagicPacket is 6 × 0xFF followed by the MAC address repeated 16 times.
func MagicPacket(mac net.HardwareAddr) ([]byte, error) {
	if len(mac) != 6 {
		return nil, fmt.Errorf("MAC must be 6 bytes, got %d", len(mac))
	}
	pkt := make([]byte, 6+16*6)
	for i := 0; i < 6; i++ {
		pkt[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(pkt[6+i*6:], mac)
	}
	return pkt, nil
}

func parseMAC(s string) (net.HardwareAddr, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "-", ":")
	s = strings.ReplaceAll(s, ".", "")
	if !strings.Contains(s, ":") && len(s) == 12 {
		raw, err := hex.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("invalid MAC %q", s)
		}
		return net.HardwareAddr(raw), nil
	}
	return net.ParseMAC(s)
}

// SendMagicPacket broadcasts a WoL packet to UDP 9 and 7.
func SendMagicPacket(mac net.HardwareAddr, broadcast string) error {
	pkt, err := MagicPacket(mac)
	if err != nil {
		return err
	}
	if broadcast == "" {
		broadcast = "255.255.255.255"
	}
	var last error
	sent := 0
	for _, port := range []int{9, 7} {
		addr := &net.UDPAddr{IP: net.ParseIP(broadcast), Port: port}
		conn, err := net.DialUDP("udp4", nil, addr)
		if err != nil {
			last = err
			continue
		}
		_, err = conn.Write(pkt)
		_ = conn.Close()
		if err != nil {
			last = err
			continue
		}
		sent++
	}
	if sent == 0 {
		if last == nil {
			last = fmt.Errorf("no broadcast sent")
		}
		return last
	}
	return nil
}

type Adapter struct {
	Name       string `json:"name"`
	MAC        string `json:"mac"`
	Addrs      string `json:"addrs"`
	Up         bool   `json:"up"`
	Loopback   bool   `json:"loopback"`
	WakeHint   string `json:"wakeHint,omitempty"`
	HardwareOK bool   `json:"hardwareOk"`
}

func listAdapters() ([]Adapter, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := make([]Adapter, 0, len(ifs))
	for _, iface := range ifs {
		if len(iface.HardwareAddr) != 6 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		parts := make([]string, 0, len(addrs))
		for _, a := range addrs {
			parts = append(parts, a.String())
		}
		out = append(out, Adapter{
			Name:       iface.Name,
			MAC:        strings.ToUpper(iface.HardwareAddr.String()),
			Addrs:      strings.Join(parts, ", "),
			Up:         iface.Flags&net.FlagUp != 0,
			HardwareOK: true,
		})
	}
	return out, nil
}
