package main

import (
	"bytes"
	"net"
	"testing"
)

func TestMagicPacket(t *testing.T) {
	mac, err := parseMAC("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatal(err)
	}
	pkt, err := MagicPacket(mac)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkt) != 102 {
		t.Fatalf("len=%d want 102", len(pkt))
	}
	if !bytes.Equal(pkt[:6], []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) {
		t.Fatalf("sync prefix: %x", pkt[:6])
	}
	for i := 0; i < 16; i++ {
		got := pkt[6+i*6 : 12+i*6]
		if !bytes.Equal(got, mac) {
			t.Fatalf("repeat %d: %x", i, got)
		}
	}
}

func TestParseMAC(t *testing.T) {
	want := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	for _, s := range []string{"AA:BB:CC:DD:EE:FF", "aa-bb-cc-dd-ee-ff", "AABBCCDDEEFF"} {
		got, err := parseMAC(s)
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s: got %s", s, got)
		}
	}
}

func TestMagicPacketRejectsShortMAC(t *testing.T) {
	if _, err := MagicPacket(net.HardwareAddr{0x01, 0x02}); err == nil {
		t.Fatal("expected error")
	}
}
