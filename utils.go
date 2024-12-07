package main

import (
	"net"
)

func getLocalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

func getRemoteIP(domain string) (string, error) {
	ips, err := net.LookupIP(domain)

	if err == nil {
		for _, ip := range ips {
			if ip.To4() != nil {
				return ip.String(), nil
			}
		}
	}

	conn, err := net.Dial("udp", domain+":80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().(*net.UDPAddr)
	return remoteAddr.IP.String(), nil
}
