package main

import (
	"fmt"
	"net/netip"
)

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
