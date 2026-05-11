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
