//go:build linux
// +build linux

package common_utils

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"os/exec"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/tun"
)

// TUNDevice 是TUN设备的实现，提供读写功能
type TUNDevice struct {
	device    tun.Device
	nativeTun *tun.NativeTun // 保存原始设备引用
	name      string
	ipAddress netip.Addr   // IP地址
	index     int          // 接口索引（Linux使用）
	link      netlink.Link // Linux网络接口
}

// SetIP 设置TUN设备的IP地址为网关IP
func (t *TUNDevice) SetIP(ipPrefix netip.Prefix) error {
	// 获取接口
	link, err := netlink.LinkByName(t.name)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %v", t.name, err)
	}
	t.link = link

	// 创建网络信息，获取网关IP
	networkInfo, err := NewNetworkInfo(ipPrefix.String())
	if err != nil {
		return fmt.Errorf("failed to create network info: %v", err)
	}

	// 获取网关前缀
	gatewayPrefix := networkInfo.GetGateway()

	// 转换为*net.IPNet
	ipNet := PrefixToIPNet(gatewayPrefix)

	addr := &netlink.Addr{
		IPNet: ipNet,
	}

	// 添加IP地址到接口
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("failed to set IP address: %v", err)
	}

	// 启用接口
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring up interface: %v", err)
	}

	t.ipAddress = gatewayPrefix.Addr()
	t.index = link.Attrs().Index

	log.Printf("Assigned gateway IP %s to TUN device %s", gatewayPrefix, t.name)
	return nil
}

// AddRoute 通过TUN设备添加路由
func (t *TUNDevice) AddRoute(prefix netip.Prefix) error {
	// 创建IP地址
	// Convert netip.Prefix to *net.IPNet
	_, dest, err := net.ParseCIDR(prefix.String())
	if err != nil {
		return fmt.Errorf("failed to parse IP prefix: %v", err)
	}
	// 创建路由
	route := &netlink.Route{
		LinkIndex: t.index,
		Dst:       dest,
		Priority:  1, // 优先级
	}

	// 添加路由
	if err := netlink.RouteAdd(route); err != nil {
		return fmt.Errorf("failed to add route: %v", err)
	}

	log.Printf("Added route: %s via %s", prefix, t.name)
	return nil
}

// CreateTunDevice 在Linux上创建和配置TUN设备
func CreateTunDevice(name string, ipPrefix netip.Prefix, mtu int) (*TUNDevice, error) {
	// 如果名称为空，则使用默认名称
	if name == "" {
		name = "masquetun"
	}

	// 创建WireGuard TUN设备
	device, err := tun.CreateTUN(name, mtu)
	if err != nil {
		return nil, fmt.Errorf("failed to create TUN device: %v", err)
	}

	// 获取接口名称
	tunName, err := device.Name()
	if err != nil {
		device.Close()
		return nil, fmt.Errorf("failed to get TUN device name: %v", err)
	}
	log.Printf("Created TUN device: %s", tunName)

	// 获取原生TUN设备
	nativeTunDevice, ok := device.(*tun.NativeTun)
	if !ok {
		device.Close()
		return nil, fmt.Errorf("failed to get native TUN device")
	}

	// 确保内核转发已启用
	enableIPForwarding()

	// 创建设备结构
	tunDevice := &TUNDevice{
		device:    device,
		nativeTun: nativeTunDevice,
		name:      tunName,
	}

	// 配置IP地址
	if err := tunDevice.SetIP(ipPrefix); err != nil {
		device.Close()
		return nil, fmt.Errorf("failed to configure TUN device IP: %v", err)
	}

	return tunDevice, nil
}

// enableIPForwarding 启用Linux内核IP转发
func enableIPForwarding() {
	// 启用IPv4转发
	cmd := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
	if err := cmd.Run(); err != nil {
		log.Printf("Warning: Failed to enable IPv4 forwarding: %v", err)
	} else {
		log.Println("IPv4 forwarding enabled")
	}

	// 启用IPv6转发
	cmd = exec.Command("sysctl", "-w", "net.ipv6.conf.all.forwarding=1")
	if err := cmd.Run(); err != nil {
		log.Printf("Warning: Failed to enable IPv6 forwarding: %v", err)
	} else {
		log.Println("IPv6 forwarding enabled")
	}
}
