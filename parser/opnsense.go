package parser

import (
	"strings"
)

// applyOPNsense post-processes a parsed message when vendorType=opnsense.
//
// Only the OPNsense `filterlog` app emits the pf CSV payload that benefits
// from rewriting; other apps (lighttpd, configd.py, openvpn, ...) are left
// with the standard RFC3164 parse.
//
// filterlog CSV layout (pf, common prefix):
//
//	0  rulenr
//	1  subrulenr
//	2  anchorname
//	3  rule_uuid
//	4  interface
//	5  reason            (match)
//	6  action            (pass|block|...)
//	7  dir               (in|out)
//	8  ipversion         (4|6)
//
// IPv4 tail (offset 9..):
//
//	9  tos
//	10 ecn
//	11 ttl
//	12 id
//	13 offset
//	14 flags             (DF|none|...)
//	15 protonum
//	16 protoname         (tcp|udp|icmp|...)
//	17 length
//	18 src
//	19 dst
//	20 srcport           (tcp/udp)
//	21 dstport           (tcp/udp)
//	22 datalen           (udp) / dataoffset (tcp)
//	23+ tcp: tcpflags, seq, ack, window, urg, options
//
// IPv6 tail (offset 9..):
//
//	9  class
//	10 flowlabel
//	11 hoplimit
//	12 protoname
//	13 protonum
//	14 length
//	15 src
//	16 dst
//	17 srcport
//	18 dstport
//	19 datalen
func applyOPNsense(msg *SyslogMessage) {
	if !strings.EqualFold(msg.AppName, "filterlog") {
		return
	}
	body := msg.Message
	if body == "" {
		return
	}
	f := strings.Split(body, ",")
	if len(f) < 9 {
		return
	}

	iface := f[4]
	action := f[6]
	dir := f[7]
	ipver := f[8]

	var proto, src, dst, srcPort, dstPort, length, tcpflags string

	switch ipver {
	case "4":
		if len(f) < 20 {
			return
		}
		proto = f[16]
		length = f[17]
		src = f[18]
		dst = f[19]
		if (proto == "tcp" || proto == "udp") && len(f) >= 22 {
			srcPort = f[20]
			dstPort = f[21]
		}
		if proto == "tcp" && len(f) >= 24 {
			tcpflags = f[23]
		}
	case "6":
		if len(f) < 17 {
			return
		}
		proto = f[12]
		length = f[14]
		src = f[15]
		dst = f[16]
		if (proto == "tcp" || proto == "udp") && len(f) >= 19 {
			srcPort = f[17]
			dstPort = f[18]
		}
		if proto == "tcp" && len(f) >= 21 {
			tcpflags = f[20]
		}
	default:
		return
	}

	var b strings.Builder
	b.WriteString(action)
	b.WriteByte(' ')
	b.WriteString(dir)
	if iface != "" {
		b.WriteByte(' ')
		b.WriteString(iface)
	}
	if proto != "" {
		b.WriteByte(' ')
		b.WriteString(proto)
	}
	b.WriteByte(' ')
	if srcPort != "" {
		b.WriteString(src)
		b.WriteByte(':')
		b.WriteString(srcPort)
	} else {
		b.WriteString(src)
	}
	b.WriteString(" -> ")
	if dstPort != "" {
		b.WriteString(dst)
		b.WriteByte(':')
		b.WriteString(dstPort)
	} else {
		b.WriteString(dst)
	}
	if tcpflags != "" {
		b.WriteString(" flags=")
		b.WriteString(tcpflags)
	}
	if length != "" {
		b.WriteString(" len=")
		b.WriteString(length)
	}

	msg.Message = b.String()
	if action != "" {
		msg.Facility = action
	}
}
