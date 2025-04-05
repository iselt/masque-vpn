//go:build linux
// +build linux

package common_utils

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"

	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
)

// TUNDevice 是TUN设备的实现，提供读写功能
type TUNDevice struct {
	device    *water.Interface // 使用water库的Interface
	name      string
	ipAddress netip.Addr // IP地址
	index     int        // 接口索引（Linux使用）
}

// Read 从TUN设备读取数据
func (t *TUNDevice) Read(buf []byte) (int, error) {
	return t.device.Read(buf)
}

// Write 向TUN设备写入数据
func (t *TUNDevice) Write(buf []byte) (int, error) {
	return t.device.Write(buf)
}

func (t *TUNDevice) Close() error {
	return t.device.Close()
}

func (t *TUNDevice) Name() string {
	return t.name
}

// 将 netip.Prefix 转换为 *net.IPNet
func prefixToIPNet(prefix netip.Prefix) *net.IPNet {
	addr := prefix.Addr()
	bits := prefix.Bits()

	var ipNet net.IPNet
	if addr.Is4() {
		ipNet = net.IPNet{
			IP:   net.IP(addr.AsSlice()),
			Mask: net.CIDRMask(bits, 32),
		}
	} else {
		ipNet = net.IPNet{
			IP:   net.IP(addr.AsSlice()),
			Mask: net.CIDRMask(bits, 128),
		}
	}
	return &ipNet
}

// SetIP 设置 TUN 设备的 IP 地址
func (t *TUNDevice) SetIP(ipPrefix netip.Prefix) error {
	// 获取设备接口
	link, err := netlink.LinkByName(t.name)
	if err != nil {
		return fmt.Errorf("failed to get device interface: %v", err)
	}

	// 保存接口索引供后续使用
	t.index = link.Attrs().Index

	// 创建 IP 地址
	ipAddr := netlink.Addr{
		IPNet: prefixToIPNet(ipPrefix),
	}

	// 设置 IP 地址
	if err := netlink.AddrAdd(link, &ipAddr); err != nil {
		if !os.IsExist(err) { // 忽略地址已存在的错误
			return fmt.Errorf("failed to set IP address: %v", err)
		}
	}

	// 启用接口
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to enable interface: %v", err)
	}

	t.ipAddress = ipPrefix.Addr() // 保存 IP 地址
	log.Printf("Assigned IP %s to TUN device %s", ipPrefix, t.name)
	return nil
}

// AddRoute 通过 TUN 设备添加路由
func (t *TUNDevice) AddRoute(prefix netip.Prefix) error {
	// 使用已保存的接口索引或重新获取
	linkIndex := t.index
	if linkIndex == 0 {
		link, err := netlink.LinkByName(t.name)
		if err != nil {
			return fmt.Errorf("failed to get device interface: %v", err)
		}
		linkIndex = link.Attrs().Index
		t.index = linkIndex
	}

	// 创建路由
	route := &netlink.Route{
		LinkIndex: linkIndex,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       prefixToIPNet(prefix),
	}

	// 添加路由
	if err := netlink.RouteAdd(route); err != nil {
		if !os.IsExist(err) { // 忽略路由已存在的错误
			return fmt.Errorf("failed to add route: %v", err)
		}
	}
	return nil
}

// SetMTU 设置 TUN 设备的 MTU 值
func (t *TUNDevice) SetMTU(mtu int) error {
	// 获取设备接口
	link, err := netlink.LinkByName(t.name)
	if err != nil {
		return fmt.Errorf("failed to get device interface: %v", err)
	}

	// 设置 MTU
	if err := netlink.LinkSetMTU(link, mtu); err != nil {
		return fmt.Errorf("failed to set MTU: %v", err)
	}

	log.Printf("Set MTU %d for TUN device %s", mtu, t.name)
	return nil
}

// CreateTunDevice 在 Linux 上创建和配置 TUN 设备
func CreateTunDevice(name string, ipPrefix netip.Prefix) (*TUNDevice, error) {
	// 检查是否有 root 权限
	if os.Geteuid() != 0 {
		log.Println("Warning: Creating TUN device may require root privileges")
	}

	// 如果名称为空，则使用默认名称
	if name == "" {
		name = "masquetun"
	}

	// 配置 water TUN 设备
	config := water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			Name: name,
		},
	}

	// 创建设备
	device, err := water.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create TUN device: %v", err)
	}

	// 创建设备结构
	tunDevice := &TUNDevice{
		device: device,
		name:   device.Name(),
	}

	log.Printf("Created TUN device successfully: %s", tunDevice.name)

	// 配置 IP 地址
	if err := tunDevice.SetIP(ipPrefix); err != nil {
		device.Close()
		return nil, fmt.Errorf("failed to configure TUN device IP: %v", err)
	}

	mtu := 1400
	// 如果指定了MTU，则设置MTU
	if mtu > 0 {
		if err := tunDevice.SetMTU(mtu); err != nil {
			device.Close()
			return nil, fmt.Errorf("failed to set TUN device MTU: %v", err)
		}
	}

	return tunDevice, nil
}

// AddRoute 为指定 TUN 设备添加路由（全局函数，用于兼容）
func AddRoute(tunDevice *TUNDevice, prefix netip.Prefix) error {
	return tunDevice.AddRoute(prefix)
}
