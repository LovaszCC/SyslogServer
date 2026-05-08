package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type SyslogMessage struct {
	Timestamp time.Time
	Hostname  string
	AppName   string
	Facility  int
	Severity  int
	Message   string
	Raw       string
}

func Parse(raw string) (*SyslogMessage, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty message")
	}

	// Default facility=user(1), severity=notice(5) when PRI absent
	facility := 1
	severity := 5
	rest := raw

	if raw[0] == '<' {
		closeBracket := strings.Index(raw, ">")
		if closeBracket < 0 {
			return nil, fmt.Errorf("malformed priority")
		}
		pri, err := strconv.Atoi(raw[1:closeBracket])
		if err != nil {
			return nil, fmt.Errorf("invalid priority: %w", err)
		}
		facility = pri / 8
		severity = pri % 8
		rest = raw[closeBracket+1:]
	}

	msg := &SyslogMessage{
		Facility: facility,
		Severity: severity,
		Raw:      raw,
	}

	// RFC 5424 starts with version "1" after the priority
	if len(rest) > 1 && rest[0] == '1' && rest[1] == ' ' {
		parseRFC5424(rest, msg)
	} else {
		parseRFC3164(rest, msg)
	}

	return msg, nil
}

// parseRFC5424 parses: 1 TIMESTAMP HOSTNAME APP-NAME PROCID MSGID [SD] MSG
func parseRFC5424(data string, msg *SyslogMessage) {
	// Skip version "1 "
	data = data[2:]

	parts := splitN(data, 7)

	// Timestamp
	if len(parts) > 0 && parts[0] != "-" {
		if t, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
			msg.Timestamp = t
		} else if t, err := time.Parse(time.RFC3339, parts[0]); err == nil {
			msg.Timestamp = t
		}
	}

	// Hostname
	if len(parts) > 1 && parts[1] != "-" {
		msg.Hostname = parts[1]
	}

	// App-Name
	if len(parts) > 2 && parts[2] != "-" {
		msg.AppName = parts[2]
	}

	// Message is everything after the structured data fields
	// Parts: timestamp, hostname, app-name, procid, msgid, SD, message...
	if len(parts) > 5 {
		// Find message portion - skip structured data
		sdAndMsg := parts[5]
		if strings.HasPrefix(sdAndMsg, "[") {
			// Find end of structured data
			depth := 0
			end := 0
			for i, ch := range sdAndMsg {
				if ch == '[' {
					depth++
				} else if ch == ']' {
					depth--
					if depth == 0 {
						end = i + 1
						break
					}
				}
			}
			if end < len(sdAndMsg) {
				msg.Message = strings.TrimSpace(sdAndMsg[end:])
			}
		} else {
			// No structured data (NILVALUE "-" or directly message)
			if sdAndMsg == "-" && len(parts) > 6 {
				msg.Message = parts[6]
			} else if sdAndMsg != "-" {
				msg.Message = sdAndMsg
			}
		}
	}

	// If message still empty, try to get remaining text
	if msg.Message == "" && len(parts) > 6 {
		msg.Message = parts[6]
	}
}

// parseRFC3164 parses: TIMESTAMP HOSTNAME APP-NAME[PID]: MSG
// or: TIMESTAMP HOSTNAME MSG
func parseRFC3164(data string, msg *SyslogMessage) {
	// Try to parse BSD timestamp: "Mon DD HH:MM:SS" or "Mon  D HH:MM:SS"
	if len(data) >= 15 {
		tsStr := data[:15]
		now := time.Now()
		layouts := []string{
			"Jan  2 15:04:05",
			"Jan 2 15:04:05",
			"Jan 02 15:04:05",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, tsStr); err == nil {
				// BSD syslog doesn't include year, use current year
				msg.Timestamp = t.AddDate(now.Year(), 0, 0)
				data = strings.TrimSpace(data[15:])
				break
			}
		}
	}

	// If no BSD timestamp was parsed, try ISO timestamp
	if msg.Timestamp.IsZero() {
		if spaceIdx := strings.Index(data, " "); spaceIdx > 0 {
			tsStr := data[:spaceIdx]
			if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
				msg.Timestamp = t
				data = strings.TrimSpace(data[spaceIdx+1:])
			}
		}
	}

	// Next token is hostname
	if spaceIdx := strings.Index(data, " "); spaceIdx > 0 {
		msg.Hostname = data[:spaceIdx]
		data = strings.TrimSpace(data[spaceIdx+1:])
	} else {
		msg.Message = data
		return
	}

	// Try to extract app name from "appname:" or "appname[pid]:" pattern
	if colonIdx := strings.Index(data, ":"); colonIdx > 0 {
		tag := data[:colonIdx]
		// Strip PID if present: "appname[1234]" -> "appname"
		if bracketIdx := strings.Index(tag, "["); bracketIdx > 0 {
			msg.AppName = tag[:bracketIdx]
		} else if !strings.Contains(tag, " ") {
			msg.AppName = tag
		}
		msg.Message = strings.TrimSpace(data[colonIdx+1:])
	} else {
		msg.Message = data
	}
}

// splitN splits a string by spaces into at most n parts.
// The last part contains the remainder of the string.
func splitN(s string, n int) []string {
	parts := make([]string, 0, n)
	remaining := s
	for i := 0; i < n-1; i++ {
		idx := strings.Index(remaining, " ")
		if idx < 0 {
			break
		}
		parts = append(parts, remaining[:idx])
		remaining = remaining[idx+1:]
	}
	parts = append(parts, remaining)
	return parts
}
