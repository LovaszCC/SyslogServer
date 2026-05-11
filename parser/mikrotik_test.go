package parser

import "testing"

func TestMikroTikParse(t *testing.T) {
	raw := `<14>1 2026-05-11T10:00:00Z router1 - - - - 0|MikroTik|CHR QEMU Standard PC (Q35 + ICH9, 2009)|7.20 (stable)|81|wireguard,debug|Low|dvchost=r-alphazoo-szfv-main-2 dvc=10.1.1.14 msg=AZWG4: [r-alphavet-aote] xaCnXzdvmw3pjYzJ5KxcOhVndWPbS+Owz2etocbUHk0\=: Receiving keepalive packet from peer (109.74.57.32:13231)`

	msg, err := Parse(raw, "mikrotik")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if msg.Facility != "wireguard" {
		t.Errorf("Facility = %q, want %q", msg.Facility, "wireguard")
	}
	if msg.Severity != "Low" {
		t.Errorf("Severity = %q, want %q", msg.Severity, "Low")
	}
	wantPrefix := "AZWG4: [r-alphavet-aote]"
	if len(msg.Message) < len(wantPrefix) || msg.Message[:len(wantPrefix)] != wantPrefix {
		t.Errorf("Message = %q, want prefix %q", msg.Message, wantPrefix)
	}
}

func TestVPNKeepsSD(t *testing.T) {
	raw := "<134>1 2026-05-11T08:43:48.661Z vpnsrv vpn-management 2754235 vpn.disconnect [vpn@32473 common_name=\"koczor2\" vpn_ip=\"10.214.12.181\" client_ip=\"39.144.89.149\" bytes_received=\"0\" bytes_sent=\"0\" rules_removed=\"2\"] \ufeffVPN disconnect koczor2 (10.214.12.181) duration=0s"
	msg, err := Parse(raw, "vpn")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wantPrefix := `[vpn@32473 common_name="koczor2"`
	if !contains(msg.Message, wantPrefix) {
		t.Errorf("Message = %q, want prefix %q", msg.Message, wantPrefix)
	}
	if msg.Hostname != "vpnsrv" {
		t.Errorf("Hostname = %q", msg.Hostname)
	}
	if msg.AppName != "vpn-management" {
		t.Errorf("AppName = %q", msg.AppName)
	}
	// PRI=134 → facility=16 severity=6
	if msg.Facility != "16" || msg.Severity != "6" {
		t.Errorf("got fac=%q sev=%q want 16/6", msg.Facility, msg.Severity)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestNoVendorKeepsRFCValues(t *testing.T) {
	raw := `<13>Oct 11 22:14:15 host1 app1: hello world`
	msg, err := Parse(raw, "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// PRI=13 → facility=1 severity=5
	if msg.Facility != "1" || msg.Severity != "5" {
		t.Errorf("got fac=%q sev=%q want 1/5", msg.Facility, msg.Severity)
	}
	if msg.Message != "hello world" {
		t.Errorf("Message = %q", msg.Message)
	}
}
