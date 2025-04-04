//go:build linux

package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"syscall"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"
)

// htons 将一个短整型从主机字节序转换为网络字节序。
func htons(host uint16) uint16 {
	return (host<<8)&0xff00 | (host>>8)&0xff
}

// createReceiveSocket 创建一个绑定到特定接口的原始套接字，用于接收 IP 数据包。
func createReceiveSocket(ifaceName string, isIPv6 bool) (int, error) {
	proto := unix.ETH_P_IP
	if isIPv6 {
		proto = unix.ETH_P_IPV6
	}
	// 使用 SOCK_DGRAM 作为 AF_PACKET 以获取已移除链路层报头的包
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM, int(htons(uint16(proto))))
	if err != nil {
		return -1, fmt.Errorf("creating AF_PACKET socket: %w", err)
	}

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		syscall.Close(fd)
		return -1, fmt.Errorf("interface lookup for '%s' failed: %w", ifaceName, err)
	}

	addr := &syscall.SockaddrLinklayer{
		Protocol: htons(uint16(proto)),
		Ifindex:  iface.Index,
		// Pktsrc 和 Hatype 可以为 0
	}
	if err := syscall.Bind(fd, addr); err != nil {
		syscall.Close(fd)
		return -1, fmt.Errorf("binding AF_PACKET socket to interface '%s' failed: %w", ifaceName, err)
	}
	log.Printf("Raw receive socket (AF_PACKET) created and bound to %s (ifindex %d)", ifaceName, iface.Index)
	return fd, nil
}

// createSendSocket 创建一个原始 IP 套接字，用于发送具有指定源 IP 的数据包。
func createSendSocket(srcAddr netip.Addr) (int, error) {
	if srcAddr.Is4() {
		return createSendSocketIPv4(srcAddr)
	}
	if srcAddr.Is6() {
		return createSendSocketIPv6(srcAddr)
	}
	return -1, fmt.Errorf("cannot create send socket for invalid source address: %s", srcAddr)
}

func createSendSocketIPv4(srcAddr netip.Addr) (int, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_RAW)
	if err != nil {
		return -1, fmt.Errorf("creating IPv4 raw socket: %w", err)
	}

	// IP_HDRINCL 意味着我们提供完整的 IP 报头
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_HDRINCL, 1); err != nil {
		unix.Close(fd)
		return -1, fmt.Errorf("setting IP_HDRINCL on IPv4 raw socket: %w", err)
	}

	// 将套接字绑定到源 IP 地址。这确保了如果内核需要填充，传出数据包具有此源 IP（尽管 IP_HDRINCL 意味着我们应该这样做）。
	// 它也可能有助于路由决策。
	sa := &unix.SockaddrInet4{Port: 0} // Port is ignored for RAW // 端口对于 RAW 被忽略
	copy(sa.Addr[:], srcAddr.AsSlice())
	if err := unix.Bind(fd, sa); err != nil {
		unix.Close(fd)
		return -1, fmt.Errorf("binding IPv4 raw socket to source addr %s: %w", srcAddr, err)
	}
	log.Printf("Raw send socket (IPv4) created and bound to %s", srcAddr)
	return fd, nil
}

func createSendSocketIPv6(srcAddr netip.Addr) (int, error) {
	fd, err := unix.Socket(unix.AF_INET6, unix.SOCK_RAW, unix.IPPROTO_RAW)
	if err != nil {
		return -1, fmt.Errorf("creating IPv6 raw socket: %w", err)
	}

	// IPV6_HDRINCL 不像 IP_HDRINCL 那样是标准的。内核通常处理报头。
	// 我们将提供完整的报头，并希望内核不会破坏它。
	// 此处使用 SOCK_RAW 的主要原因通常是为了绕过 netfilter。
	// 绑定仍然有用。

	sa := &unix.SockaddrInet6{Port: 0} // 端口被忽略
	copy(sa.Addr[:], srcAddr.AsSlice())
	if err := unix.Bind(fd, sa); err != nil {
		unix.Close(fd)
		return -1, fmt.Errorf("binding IPv6 raw socket to source addr %s: %w", srcAddr, err)
	}
	log.Printf("Raw send socket (IPv6) created and bound to %s", srcAddr)
	return fd, nil
}

// sendOnSocket 使用适当的原始套接字 syscall 发送 IP 数据包。
func sendOnSocket(fd int, b []byte) error {
	if len(b) == 0 {
		return errors.New("cannot send empty packet")
	}
	if fd < 0 {
		return errors.New("invalid send socket file descriptor")
	}

	version := ipVersion(b)
	switch version {
	case 4:
		if len(b) < ipv4.HeaderLen {
			return errors.New("IPv4 packet too short")
		}
		dest := ([4]byte)(b[16:20]) // IPv4 报头中的目标 IP 地址偏移量
		sa := &unix.SockaddrInet4{Addr: dest}
		// log.Printf("Sending %d bytes (IPv4) via raw socket to %s", len(b), netip.AddrFrom4(dest).String())
		err := unix.Sendto(fd, b, 0, sa)
		if err != nil {
			return fmt.Errorf("sendto IPv4 packet failed: %w", err)
		}
		return nil
	case 6:
		if len(b) < ipv6.HeaderLen {
			return errors.New("IPv6 packet too short")
		}
		dest := ([16]byte)(b[24:40]) // IPv6 报头中的目标 IP 地址偏移量
		sa := &unix.SockaddrInet6{Addr: dest}
		// log.Printf("Sending %d bytes (IPv6) via raw socket to %s", len(b), netip.AddrFrom16(dest).String())
		err := unix.Sendto(fd, b, 0, sa)
		if err != nil {
			return fmt.Errorf("sendto IPv6 packet failed: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown IP version %d in packet header", version)
	}
}

// ipVersion 从数据包的第一个字节中提取 IP 版本（4 或 6）。
func ipVersion(b []byte) uint8 {
	if len(b) < 1 {
		return 0 // 无效
	}
	return b[0] >> 4
}
