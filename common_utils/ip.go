package common_utils

import (
	"fmt"
	"net"
	"net/netip"
)

// PrefixToIPNet converts a netip.Prefix to a *net.IPNet
func PrefixToIPNet(prefix netip.Prefix) *net.IPNet {
	bits := prefix.Bits()
	addr := prefix.Addr()

	var ip net.IP
	var mask net.IPMask

	if addr.Is4() {
		// 对IPv4直接使用4字节表示，避免16字节分配
		ipv4 := addr.As4()
		ip = net.IPv4(ipv4[0], ipv4[1], ipv4[2], ipv4[3]).To4()
		mask = net.CIDRMask(bits, 32)
	} else {
		// IPv6
		ip = net.IP(addr.AsSlice()) // 使用AsSlice()避免复制
		mask = net.CIDRMask(bits, 128)
	}

	return &net.IPNet{IP: ip, Mask: mask}
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

// NetworkInfo 仅保存网络配置信息，不负责分配
type NetworkInfo struct {
	prefix  netip.Prefix // 整个 VPN 网段
	gateway netip.Prefix // 网关 IP
}

// NewNetworkInfo 创建网络信息对象
func NewNetworkInfo(cidrStr string) (*NetworkInfo, error) {
	prefix, err := netip.ParsePrefix(cidrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR %s: %v", cidrStr, err)
	}

	// 获取网络前缀(清除主机位)
	networkPrefix := prefix.Masked()

	// 计算第一个 IP (网关)
	firstIP := nextIP(networkPrefix.Addr())

	// 创建网关的前缀（与网络使用相同的掩码）
	gatewayPrefix := netip.PrefixFrom(firstIP, networkPrefix.Bits())

	return &NetworkInfo{
		prefix:  networkPrefix,
		gateway: gatewayPrefix,
	}, nil
}

// 获取网关 IP
func (n *NetworkInfo) GetGateway() netip.Prefix {
	return n.gateway
}

// 获取网络前缀
func (n *NetworkInfo) GetPrefix() netip.Prefix {
	return n.prefix
}

// 生成下一个 IP 地址
func nextIP(ip netip.Addr) netip.Addr {
	bytes := ip.AsSlice()

	// 从最低字节开始加 1，处理进位
	for i := len(bytes) - 1; i >= 0; i-- {
		bytes[i]++
		if bytes[i] != 0 { // 如果没有溢出
			break
		}
	}

	next, _ := netip.AddrFromSlice(bytes)
	return next
}
