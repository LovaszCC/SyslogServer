package parser

import "strings"

// applyUniFi post-processes a parsed message when vendorType=unifi.
//
// UniFi emits two distinct shapes:
//
//  1. UniFi Network Controller — a CEF record carried in an RFC3164 frame:
//
//     May 14 12:25:22 UniFi-Controller CEF:0|Ubiquiti|UniFi Network|10.0.162|546|Admin Made Config Changes|2|src=... msg=...
//
//     The generic parser sets AppName="CEF" and leaves the pipe-delimited
//     CEF body in Message. CEF layout:
//       0 version | 1 vendor | 2 product | 3 dev-version | 4 signature-id |
//       5 name | 6 severity | 7 extension (key=value, ... msg=<text>)
//     We override AppName with the CEF Name, Severity with the CEF severity,
//     and Message with the text after `msg=` in the extension.
//
//  2. UniFi APs/switches — RFC3164 with a `<MAC>,<MODEL-FW>:` device prefix:
//
//     <13>May 14 12:25:27 acc-storage-tolto 6c63f8356535,USW-Lite-8-PoE-7.4.1+16850: syswrapper[27473]: Provision took 3 sec, full=0
//
//     The generic parser captures the whole `<MAC>,<MODEL-FW>` token as
//     AppName and leaves `<app>[pid]: <text>` in Message. We move the model
//     into Facility, then re-extract the real app tag and message body.
//
// Records that match neither shape are left as parsed by the generic parser.
func applyUniFi(msg *SyslogMessage) {
	if strings.EqualFold(msg.AppName, "CEF") || strings.HasPrefix(msg.Message, "CEF:") {
		applyUniFiCEF(msg)
		return
	}
	if mac, model, ok := splitDevicePrefix(msg.AppName); ok {
		applyUniFiDevice(msg, mac, model)
	}
}

func applyUniFiCEF(msg *SyslogMessage) {
	body := msg.Message
	body = strings.TrimPrefix(body, "CEF:")
	parts := strings.Split(body, "|")
	if len(parts) < 8 || !strings.EqualFold(parts[1], "Ubiquiti") {
		return
	}

	if name := parts[5]; name != "" {
		msg.AppName = name
	}
	if sev := parts[6]; sev != "" {
		msg.Severity = sev
	}

	ext := parts[7]
	if idx := strings.Index(ext, "msg="); idx >= 0 {
		msg.Message = strings.TrimSpace(ext[idx+4:])
	} else {
		msg.Message = strings.TrimSpace(ext)
	}
}

func applyUniFiDevice(msg *SyslogMessage, mac, model string) {
	if model != "" {
		msg.Facility = model
	}

	body := strings.TrimSpace(msg.Message)
	// Some records carry an empty leading tag: ": cfgmtd[2393]: ..."
	body = strings.TrimSpace(strings.TrimPrefix(body, ":"))

	colon := strings.IndexByte(body, ':')
	if colon <= 0 {
		msg.AppName = mac
		msg.Message = body
		return
	}

	tag := body[:colon]
	if strings.Contains(tag, " ") {
		// Not an app tag (e.g. a kernel timestamp line); leave body intact.
		msg.AppName = mac
		msg.Message = body
		return
	}

	if bracket := strings.IndexByte(tag, '['); bracket > 0 {
		tag = tag[:bracket]
	}
	msg.AppName = tag
	msg.Message = strings.TrimSpace(body[colon+1:])
}

// splitDevicePrefix recognises a UniFi device token "<MAC>,<MODEL-FW>".
// The MAC is 12 hex chars; the model is everything after the first comma.
func splitDevicePrefix(s string) (mac, model string, ok bool) {
	comma := strings.IndexByte(s, ',')
	if comma != 12 {
		return "", "", false
	}
	mac = s[:comma]
	for _, c := range mac {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return "", "", false
		}
	}
	return mac, s[comma+1:], true
}
