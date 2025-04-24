package common_utils

import (
	"fmt"
	"net"
	"net/netip"
	"sync"
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

// GetIPAddresses 从IP包中提取源IP和目标IP地址
func GetIPAddresses(packet []byte, length int) (src, dst netip.Addr, err error) {
	if length < 20 { // IPv4头部最小长度为20字节
		return netip.Addr{}, netip.Addr{}, fmt.Errorf("packet length too short (%d bytes), not a valid IP packet", length)
	}

	// 检查IP版本(版本号在第一个字节的高4位)
	version := int(packet[0] >> 4)

	switch version {
	case 4: // IPv4
		if length < 20 { // IPv4最小头部长度
			return netip.Addr{}, netip.Addr{}, fmt.Errorf("IPv4 packet too short (%d bytes)", length)
		}

		// 源IP在字节12-15，目标IP在字节16-19
		srcIP, ok := netip.AddrFromSlice(packet[12:16])
		if !ok {
			return netip.Addr{}, netip.Addr{}, fmt.Errorf("invalid source IPv4 address")
		}

		dstIP, ok := netip.AddrFromSlice(packet[16:20])
		if !ok {
			return netip.Addr{}, netip.Addr{}, fmt.Errorf("invalid destination IPv4 address")
		}

		return srcIP, dstIP, nil

	case 6: // IPv6
		if length < 40 { // IPv6标准头部长度
			return netip.Addr{}, netip.Addr{}, fmt.Errorf("IPv6 packet too short (%d bytes)", length)
		}

		// 源IP在字节8-23，目标IP在字节24-39
		srcIP, ok := netip.AddrFromSlice(packet[8:24])
		if !ok {
			return netip.Addr{}, netip.Addr{}, fmt.Errorf("invalid source IPv6 address")
		}

		dstIP, ok := netip.AddrFromSlice(packet[24:40])
		if !ok {
			return netip.Addr{}, netip.Addr{}, fmt.Errorf("invalid destination IPv6 address")
		}

		return srcIP, dstIP, nil

	default:
		return netip.Addr{}, netip.Addr{}, fmt.Errorf("unsupported IP version: %d", version)
	}
}

// GetSourceIP 从IP包中提取源IP地址
func GetSourceIP(packet []byte, length int) (netip.Addr, error) {
	src, _, err := GetIPAddresses(packet, length)
	return src, err
}

// GetDestinationIP 从IP包中提取目标IP地址
func GetDestinationIP(packet []byte, length int) (netip.Addr, error) {
	_, dst, err := GetIPAddresses(packet, length)
	return dst, err
}

// ------------------ IP 地址池（IPAM）实现 ------------------

// IPPool 用于动态分配和回收 IP 地址
// 线程安全
// 仅支持 /24 及更小子网（IPv4），IPv6 也支持
// 分配时跳过网关和网络地址

type IPPool struct {
	prefix    netip.Prefix
	gateway   netip.Addr
	allocated map[netip.Addr]string // IP -> clientID
	available []netip.Addr
	mu        sync.Mutex
}

// NewIPPool 创建 IP 地址池，自动跳过网关和网络地址
func NewIPPool(prefix netip.Prefix, gateway netip.Addr) *IPPool {
	ips := []netip.Addr{}
	start := nextIP(prefix.Addr())
	end := LastIP(prefix)
	for ip := start; ip.Compare(end) <= 0; ip = nextIP(ip) {
		if ip == gateway || ip == prefix.Addr() {
			continue // 跳过网关和网络地址
		}
		ips = append(ips, ip)
	}
	return &IPPool{
		prefix:    prefix,
		gateway:   gateway,
		allocated: make(map[netip.Addr]string),
		available: ips,
	}
}

// Allocate 分配一个未分配的 IP，返回 /32 前缀
func (p *IPPool) Allocate(clientID string) (netip.Prefix, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.available) == 0 {
		return netip.Prefix{}, fmt.Errorf("no available IP addresses")
	}
	ip := p.available[0]
	p.available = p.available[1:]
	p.allocated[ip] = clientID
	return netip.PrefixFrom(ip, 32), nil
}

// Release 释放 IP 地址
func (p *IPPool) Release(ip netip.Addr) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.allocated[ip]; ok {
		delete(p.allocated, ip)
		p.available = append(p.available, ip)
	}
}
