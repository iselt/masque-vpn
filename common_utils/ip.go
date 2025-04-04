package common_utils

import (
	"net"
	"net/netip"
)

// PrefixToIPNet converts a netip.Prefix to a *net.IPNet
func PrefixToIPNet(prefix netip.Prefix) *net.IPNet {
	bits := prefix.Bits()
	addr := prefix.Addr()

	var ipnet *net.IPNet
	if addr.Is4() {
		// IPv4
		ipv4 := addr.As4()
		ipnet = &net.IPNet{
			IP:   net.IPv4(ipv4[0], ipv4[1], ipv4[2], ipv4[3]),
			Mask: net.CIDRMask(bits, 32),
		}
	} else {
		// IPv6
		ipv6 := addr.As16()
		ipnet = &net.IPNet{
			IP:   net.IP(ipv6[:]),
			Mask: net.CIDRMask(bits, 128),
		}
	}
	return ipnet
}

// LastIP returns the last IP address in a prefix/subnet
func LastIP(prefix netip.Prefix) netip.Addr {
	addr := prefix.Addr()
	bits := prefix.Bits()

	if addr.Is4() {
		// 处理IPv4地址
		ipv4 := addr.As4()
		mask := net.CIDRMask(bits, 32)

		// 将主机部分的所有位设为1
		for i := 0; i < 4; i++ {
			ipv4[i] |= ^mask[i]
		}

		return netip.AddrFrom4(ipv4)
	} else {
		// 处理IPv6地址
		ipv6 := addr.As16()
		mask := net.CIDRMask(bits, 128)

		// 将主机部分的所有位设为1
		for i := 0; i < 16; i++ {
			ipv6[i] |= ^mask[i]
		}

		return netip.AddrFrom16(ipv6)
	}
}
