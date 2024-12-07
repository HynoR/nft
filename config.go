package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Protocol represents supported network protocols
type Protocol int

const (
	All Protocol = iota
	TCP
	UDP
)

func (p Protocol) String() string {
	switch p {
	case UDP:
		return "udp"
	case TCP:
		return "tcp"
	default:
		return "all"
	}
}

func (p Protocol) tcpPrefix() string {
	switch p {
	case UDP:
		return "#"
	default:
		return ""
	}
}

func (p Protocol) udpPrefix() string {
	switch p {
	case TCP:
		return "#"
	default:
		return ""
	}
}

// ProtocolFromString converts a string to Protocol
func ProtocolFromString(s string) Protocol {
	switch strings.ToLower(s) {
	case "udp":
		return UDP
	case "tcp":
		return TCP
	default:
		return All
	}
}

// NatCell represents different types of NAT rules
type NatCell struct {
	Type      string
	SrcPort   int
	DstPort   int
	PortStart int
	PortEnd   int
	DstDomain string
	DstIP     string
	LocalIP   string
	Protocol  Protocol
	Content   string
}

func (n *NatCell) String() string {
	switch n.Type {
	case "Range":
		return fmt.Sprintf("Range(%d-%d, %s, %s, %v)", n.PortStart, n.PortEnd, n.DstDomain, n.DstIP, n.Protocol)
	case "Single":
		return fmt.Sprintf("Single(%d-%d, %s, %s, %v)", n.SrcPort, n.DstPort, n.DstDomain, n.DstIP, n.Protocol)
	default:
		return n.Content
	}
}

func (n *NatCell) Build() string {
	if n.Type == "Comment" {
		return n.Content
	}

	switch n.Type {
	case "Range":
		return fmt.Sprintf("# %+v\n"+
			"%sadd rule ip nat PREROUTING tcp dport %d-%d counter dnat to %s:%d-%d\n"+
			"%sadd rule ip nat PREROUTING udp dport %d-%d counter dnat to %s:%d-%d\n"+
			"%sadd rule ip nat POSTROUTING ip daddr %s tcp dport %d-%d counter snat to %s\n"+
			"%sadd rule ip nat POSTROUTING ip daddr %s udp dport %d-%d counter snat to %s\n\n",
			n,
			n.Protocol.tcpPrefix(), n.PortStart, n.PortEnd, n.DstIP, n.PortStart, n.PortEnd,
			n.Protocol.udpPrefix(), n.PortStart, n.PortEnd, n.DstIP, n.PortStart, n.PortEnd,
			n.Protocol.tcpPrefix(), n.DstIP, n.PortStart, n.PortEnd, n.LocalIP,
			n.Protocol.udpPrefix(), n.DstIP, n.PortStart, n.PortEnd, n.LocalIP)

	case "Single":
		if n.DstDomain == "localhost" || n.DstDomain == "127.0.0.1" {
			return fmt.Sprintf("# %+v\n"+
				"%sadd rule ip nat PREROUTING tcp dport %d redirect to :%d\n"+
				"%sadd rule ip nat PREROUTING udp dport %d redirect to :%d\n\n",
				n,
				n.Protocol.tcpPrefix(), n.SrcPort, n.DstPort,
				n.Protocol.udpPrefix(), n.SrcPort, n.DstPort)
		}

		return fmt.Sprintf("# %+v\n"+
			"%sadd rule ip nat PREROUTING tcp dport %d counter dnat to %s:%d\n"+
			"%sadd rule ip nat PREROUTING udp dport %d counter dnat to %s:%d\n"+
			"%sadd rule ip nat POSTROUTING ip daddr %s tcp dport %d counter snat to %s\n"+
			"%sadd rule ip nat POSTROUTING ip daddr %s udp dport %d counter snat to %s\n\n",
			n,
			n.Protocol.tcpPrefix(), n.SrcPort, n.DstIP, n.DstPort,
			n.Protocol.udpPrefix(), n.SrcPort, n.DstIP, n.DstPort,
			n.Protocol.tcpPrefix(), n.DstIP, n.DstPort, n.LocalIP,
			n.Protocol.udpPrefix(), n.DstIP, n.DstPort, n.LocalIP)
	}

	return ""
}

func Example(conf string) {
	fmt.Printf("请在 %s 编写转发规则，内容类似：", conf)
	fmt.Printf("SINGLE,10000,443,baidu.com\nRANGE,1000,2000,baidu.com")
}

func ReadConfig(conf string) []NatCell {
	var natCells []NatCell

	content, err := os.ReadFile(conf)
	if err != nil {
		Example(conf)
		os.Exit(1)
	}

	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	natCells = append(natCells, NatCell{
		Type:    "Comment",
		Content: "# ------- File: " + conf + " -------\n",
	})
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#") {
			natCells = append(natCells, NatCell{
				Type:    "Comment",
				Content: line + "\n",
			})
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) < 4 || len(parts) > 5 {
			slog.Warn("rule is not valid", slog.Any("line", line))
			continue
		}

		protocol := All
		if len(parts) == 5 {
			protocol = ProtocolFromString(strings.TrimSpace(parts[4]))
		}

		switch strings.TrimSpace(parts[0]) {
		case "RANGE":
			start, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			end, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
			natCells = append(natCells, NatCell{
				Type:      "Range",
				PortStart: start,
				PortEnd:   end,
				DstDomain: strings.TrimSpace(parts[3]),
				Protocol:  protocol,
			})

		case "SINGLE":
			src, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			dst, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
			natCells = append(natCells, NatCell{
				Type:      "Single",
				SrcPort:   src,
				DstPort:   dst,
				DstDomain: strings.TrimSpace(parts[3]),
				Protocol:  protocol,
			})

		default:
			slog.Warn("rule is not valid", slog.Any("line", line))

		}
	}
	natCells = append(natCells, NatCell{
		Type:    "Comment",
		Content: "# ------- End -------\n",
	})

	return natCells
}
