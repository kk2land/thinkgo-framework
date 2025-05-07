package thinkgo

import (
	"bytes"
	"net"
)

var v4InV6Prefix = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff}

// IsIPv4 判断是否是ipv4
func IsIPv4(ip net.IP) bool {
	if len(ip) == net.IPv4len {
		return true
	}
	if len(ip) == net.IPv6len && bytes.Compare(ip[0:12], v4InV6Prefix) == 0 {
		return true
	}
	return false
}
