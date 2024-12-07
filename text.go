package main

const (
	nftablesEtc  = "/etc/nftables"
	ipForward    = "/proc/sys/net/ipv4/ip_forward"
	scriptPrefix = `#!/usr/sbin/nft -f

add table ip nat
delete table ip nat
add table ip nat
add chain nat PREROUTING { type nat hook prerouting priority -100 ; }
add chain nat POSTROUTING { type nat hook postrouting priority 100 ; }

`
)
