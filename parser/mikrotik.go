package parser

import "strings"

// applyMikroTik post-processes a parsed message when vendorType=mikrotik.
//
// Expected Message body shape (pipe-delimited):
//   0|MikroTik|<model>|<firmware>|<id>|<topics>|<severity>|<kv-payload msg=...>
//
// Overrides Facility (first topic), Severity (label), and Message (text after msg=).
// Leaves the message untouched if the pipe layout doesn't match.
func applyMikroTik(msg *SyslogMessage) {
	body := msg.Message
	if body == "" {
		return
	}

	parts := strings.Split(body, "|")
	if len(parts) < 8 || !strings.EqualFold(parts[1], "MikroTik") {
		return
	}

	topics := parts[5]
	severityLabel := parts[6]
	payload := parts[7]

	if topics != "" {
		if comma := strings.IndexByte(topics, ','); comma >= 0 {
			msg.Facility = topics[:comma]
		} else {
			msg.Facility = topics
		}
	}
	if severityLabel != "" {
		msg.Severity = severityLabel
	}

	if idx := strings.Index(payload, "msg="); idx >= 0 {
		msg.Message = strings.TrimSpace(payload[idx+4:])
	} else {
		msg.Message = strings.TrimSpace(payload)
	}
}
