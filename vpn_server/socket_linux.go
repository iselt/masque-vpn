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

// htons converts a short integer from host-to-network byte order.
func htons(host uint16) uint16 {
	return (host<<8)&0xff00 | (host>>8)&0xff
}

// createReceiveSocket creates a raw socket bound to a specific interface for receiving IP packets.
func createReceiveSocket(ifaceName string, isIPv6 bool) (int, error) {
	proto := unix.ETH_P_IP
	if isIPv6 {
		proto = unix.ETH_P_IPV6
	}
	// Use SOCK_DGRAM for AF_PACKET to get packets with link-layer header removed
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
		// Pktsrc and Hatype can be 0
	}
	if err := syscall.Bind(fd, addr); err != nil {
		syscall.Close(fd)
		return -1, fmt.Errorf("binding AF_PACKET socket to interface '%s' failed: %w", ifaceName, err)
	}
	log.Printf("Raw receive socket (AF_PACKET) created and bound to %s (ifindex %d)", ifaceName, iface.Index)
	return fd, nil
}

// createSendSocket creates a raw IP socket for sending packets with specified source IP.
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

	// IP_HDRINCL means we provide the full IP header
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_HDRINCL, 1); err != nil {
		unix.Close(fd)
		return -1, fmt.Errorf("setting IP_HDRINCL on IPv4 raw socket: %w", err)
	}

	// Bind the socket to the source IP address. This ensures outgoing packets
	// have this source IP if the kernel needs to fill it (though IP_HDRINCL means we should).
	// It might also help with routing decisions.
	sa := &unix.SockaddrInet4{Port: 0} // Port is ignored for RAW
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

	// IPV6_HDRINCL is not standard like IP_HDRINCL. Kernel usually handles header.
	// We will provide the full header anyway and hope the kernel doesn't mangle it.
	// The primary reason for SOCK_RAW here is often to bypass netfilter.
	// Binding is still useful.

	// Try setting IPV6_HDRINCL if available (may not be needed or supported)
	// err = unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_HDRINCL, 1)
	// if err != nil {
	//  log.Printf("Warning: setting IPV6_HDRINCL failed (may not be necessary): %v", err)
	// }

	sa := &unix.SockaddrInet6{Port: 0} // Port is ignored
	copy(sa.Addr[:], srcAddr.AsSlice())
	if err := unix.Bind(fd, sa); err != nil {
		unix.Close(fd)
		return -1, fmt.Errorf("binding IPv6 raw socket to source addr %s: %w", srcAddr, err)
	}
	log.Printf("Raw send socket (IPv6) created and bound to %s", srcAddr)
	return fd, nil
}

// sendOnSocket sends an IP packet using the appropriate raw socket syscall.
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
		dest := ([4]byte)(b[16:20]) // Destination IP address offset in IPv4 header
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
		dest := ([16]byte)(b[24:40]) // Destination IP address offset in IPv6 header
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

// ipVersion extracts the IP version (4 or 6) from the first byte of a packet.
func ipVersion(b []byte) uint8 {
	if len(b) < 1 {
		return 0 // Invalid
	}
	return b[0] >> 4
}
