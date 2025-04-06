package common_utils

import (
	"net/netip"
)

// Read 从TUN设备读取数据，使用原生接口直接传递参数
func (t *TUNDevice) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	return t.device.Read(bufs, sizes, offset)
}

// ReadPacket 从TUN设备读取单个数据包
func (t *TUNDevice) ReadPacket(packet []byte, offset int) (int, error) {
	// 包装为[][]byte格式以适配底层接口
	bufs := [][]byte{packet}
	sizes := make([]int, 1)

	n, err := t.device.Read(bufs, sizes, offset)
	if err != nil {
		return 0, err
	}

	if n > 0 {
		return sizes[0], nil // 返回实际读取的字节数
	}
	return 0, nil
}

// Write 向TUN设备写入数据，使用原生接口直接传递参数
func (t *TUNDevice) Write(bufs [][]byte, offset int) (int, error) {
	return t.device.Write(bufs, offset)
}

func (t *TUNDevice) WritePacket(packet []byte, offset int) (int, error) {
	// 包装为[][]byte格式以适配底层接口
	bufs := [][]byte{packet}
	n, err := t.device.Write(bufs, offset)
	if err != nil {
		return 0, err
	}

	return n, nil
}

func (t *TUNDevice) Close() error {
	return t.device.Close()
}

func (t *TUNDevice) Name() string {
	return t.name
}

func (t *TUNDevice) BatchSize() int {
	return t.device.BatchSize()
}

// AddRoute 为指定TUN设备添加路由（全局函数，用于兼容）
func AddRoute(tunDevice *TUNDevice, prefix netip.Prefix) error {
	return tunDevice.AddRoute(prefix)
}
