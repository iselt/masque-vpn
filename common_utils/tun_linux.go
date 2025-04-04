//go:build linux
// +build linux

package common_utils

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/tun"
)

// TUNDevice 是TUN设备的实现，提供读写功能
type TUNDevice struct {
	device    tun.Device
	nativeTun *tun.NativeTun // 保存原始设备引用
	name      string
	ipAddress netip.Addr // IP地址
	index     int        // 接口索引（Linux使用）
}

// Read 实现tun.Device接口，直接转发到底层设备
func (t *TUNDevice) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	return t.device.Read(bufs, sizes, offset)
}

// Write 实现tun.Device接口，直接转发到底层设备
func (t *TUNDevice) Write(bufs [][]byte, offset int) (int, error) {
	return t.device.Write(bufs, offset)
}

func (t *TUNDevice) Close() error {
	return t.device.Close()
}

func (t *TUNDevice) Name() string {
	return t.name
}

// LUID 在 Linux 上不适用，返回 0
func (t *TUNDevice) LUID() uint64 {
	return 0
}

func (t *TUNDevice) BatchSize() int {
	return t.device.BatchSize()
}

// 新增：将 netip.Prefix 转换为 *net.IPNet
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
		IPNet: prefixToIPNet(ipPrefix), // 使用辅助函数
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
		Dst:       prefixToIPNet(prefix), // 使用辅助函数
	}

	// 添加路由
	if err := netlink.RouteAdd(route); err != nil {
		if !os.IsExist(err) { // 忽略路由已存在的错误
			return fmt.Errorf("failed to add route: %v", err)
		}
	}
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

	// 创建 WireGuard TUN 设备
	device, err := tun.CreateTUN(name, 1360)
	if err != nil {
		return nil, fmt.Errorf("failed to create TUN device: %v", err)
	}

	// 获取接口名称
	tunName, err := device.Name()
	if err != nil {
		device.Close()
		return nil, fmt.Errorf("failed to get TUN device name: %v", err)
	}
	log.Printf("Created TUN device successfully: %s", tunName)

	// 获取原生 TUN 设备
	nativeTunDevice, ok := device.(*tun.NativeTun)
	if !ok {
		device.Close()
		return nil, fmt.Errorf("failed to get native TUN device")
	}

	// 创建设备结构
	tunDevice := &TUNDevice{
		device:    device,
		nativeTun: nativeTunDevice,
		name:      tunName,
	}

	// 配置 IP 地址
	if err := tunDevice.SetIP(ipPrefix); err != nil {
		device.Close()
		return nil, fmt.Errorf("failed to configure TUN device IP: %v", err)
	}

	return tunDevice, nil
}

// AddRoute 为指定 TUN 设备添加路由（全局函数，用于兼容）
func AddRoute(tunDevice *TUNDevice, prefix netip.Prefix) error {
	return tunDevice.AddRoute(prefix)
}
