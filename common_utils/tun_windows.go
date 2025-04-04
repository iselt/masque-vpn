//go:build windows
// +build windows

package common_utils

import (
	"fmt"
	"log"
	"net/netip"

	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

// TUNDevice 是TUN设备的实现，提供读写功能
type TUNDevice struct {
	device    tun.Device
	nativeTun *tun.NativeTun // 保存原始设备引用
	name      string
	ipAddress netip.Addr    // IP地址
	index     int           // 接口索引（Linux使用）
	luid      winipcfg.LUID // LUID（Windows使用）
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

// LUID 返回Windows网络接口的LUID
func (t *TUNDevice) LUID() uint64 {
	return uint64(t.luid)
}

func (t *TUNDevice) BatchSize() int {
	return t.device.BatchSize()
}

// SetIP 设置TUN设备的IP地址
func (t *TUNDevice) SetIP(ipPrefix netip.Prefix) error {
	err := t.luid.SetIPAddresses([]netip.Prefix{ipPrefix})
	if err != nil {
		return fmt.Errorf("failed to set IP address: %v", err)
	}
	t.ipAddress = ipPrefix.Addr()
	log.Printf("Assigned IP %s to TUN device %s", ipPrefix, t.name)
	return nil
}

// AddRoute 通过TUN设备添加路由
func (t *TUNDevice) AddRoute(prefix netip.Prefix) error {
	nextHop := t.ipAddress
	metric := uint32(1)

	err := t.luid.AddRoute(prefix, nextHop, metric)
	if err != nil {
		return fmt.Errorf("failed to add route: %v", err)
	}

	log.Printf("Added route: %s via %s", prefix, t.name)
	return nil
}

// CreateTunDevice 在Windows上创建和配置TUN设备
func CreateTunDevice(name string, ipPrefix netip.Prefix) (*TUNDevice, error) {
	// 如果名称为空，则使用默认名称
	if name == "" {
		name = "masquetun"
	}

	// 创建WireGuard TUN设备
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
	log.Printf("Created TUN device: %s", tunName)

	// 获取原生TUN设备
	nativeTunDevice, ok := device.(*tun.NativeTun)
	if !ok {
		device.Close()
		return nil, fmt.Errorf("failed to get native TUN device")
	}

	luid := winipcfg.LUID(nativeTunDevice.LUID())

	// 创建设备结构
	tunDevice := &TUNDevice{
		device:    device,
		nativeTun: nativeTunDevice,
		name:      tunName,
		luid:      luid,
	}

	// 配置IP地址
	if err := tunDevice.SetIP(ipPrefix); err != nil {
		device.Close()
		return nil, fmt.Errorf("failed to configure TUN device IP: %v", err)
	}

	return tunDevice, nil
}

// AddRoute 为指定TUN设备添加路由（全局函数，用于兼容）
func AddRoute(tunDevice *TUNDevice, prefix netip.Prefix) error {
	return tunDevice.AddRoute(prefix)
}
