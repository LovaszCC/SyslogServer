package parser

import "strings"

// applyVPN keeps the RFC5424 structured-data segment as part of the Message.
// Standard parsing strips the [sd-id ...] block; for the VPN vendor we want
// both the SD block and the trailing free-text in the Message column.
func applyVPN(msg *SyslogMessage) {
	if idx := strings.Index(msg.Raw, "["); idx >= 0 {
		msg.Message = msg.Raw[idx:]
	}
}
