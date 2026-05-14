package parser

import "testing"

func TestUniFiCEF(t *testing.T) {
	raw := `May 14 12:25:22 UniFi-Controller CEF:0|Ubiquiti|UniFi Network|10.0.162|546|Admin Made Config Changes|2|src=10.1.14.42 UNIFIcategory=System UNIFIsubCategory=Admin UNIFIadmin=sepsigav msg=sepsigav made 2 changes to System settings. Source IP: 10.1.14.42`

	msg, err := Parse(raw, "unifi")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if msg.Hostname != "UniFi-Controller" {
		t.Errorf("Hostname = %q", msg.Hostname)
	}
	if msg.AppName != "Admin Made Config Changes" {
		t.Errorf("AppName = %q", msg.AppName)
	}
	if msg.Severity != "2" {
		t.Errorf("Severity = %q, want 2", msg.Severity)
	}
	want := "sepsigav made 2 changes to System settings. Source IP: 10.1.14.42"
	if msg.Message != want {
		t.Errorf("Message = %q, want %q", msg.Message, want)
	}
}

func TestUniFiDevice(t *testing.T) {
	raw := `<13>May 14 12:25:27 acc-storage-tolto 6c63f8356535,USW-Lite-8-PoE-7.4.1+16850: syswrapper[27473]: Provision took 3 sec, full=0`

	msg, err := Parse(raw, "unifi")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if msg.Hostname != "acc-storage-tolto" {
		t.Errorf("Hostname = %q", msg.Hostname)
	}
	if msg.AppName != "syswrapper" {
		t.Errorf("AppName = %q, want syswrapper", msg.AppName)
	}
	if msg.Facility != "USW-Lite-8-PoE-7.4.1+16850" {
		t.Errorf("Facility = %q", msg.Facility)
	}
	if msg.Message != "Provision took 3 sec, full=0" {
		t.Errorf("Message = %q", msg.Message)
	}
}

func TestUniFiDeviceEmptyTag(t *testing.T) {
	raw := `<14>May 14 12:25:30 acc-aska-sw ac8ba960f979,USW-Enterprise-8-PoE-7.4.1+16850: : cfgmtd[2393]: cfgmtd.cfgmtd_do_write(): Found Backup1 on[1]`

	msg, err := Parse(raw, "unifi")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if msg.AppName != "cfgmtd" {
		t.Errorf("AppName = %q, want cfgmtd", msg.AppName)
	}
	if msg.Message != "cfgmtd.cfgmtd_do_write(): Found Backup1 on[1]" {
		t.Errorf("Message = %q", msg.Message)
	}
}

func TestUniFiDeviceNestedTag(t *testing.T) {
	raw := `<30>May 14 12:25:27 acc-storage-tolto 6c63f8356535,USW-Lite-8-PoE-7.4.1+16850: mcad: mcad[416]: ace_reporter.set_inform_family(): next inform family: ipv4`

	msg, err := Parse(raw, "unifi")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if msg.AppName != "mcad" {
		t.Errorf("AppName = %q, want mcad", msg.AppName)
	}
	if msg.Message != "mcad[416]: ace_reporter.set_inform_family(): next inform family: ipv4" {
		t.Errorf("Message = %q", msg.Message)
	}
}
