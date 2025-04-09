package common_utils

import (
	"errors"
	"net/netip"
)

// Read 从TUN设备读取数据，使用原生接口直接传递参数
func (t *TUNDevice) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	return t.device.Read(bufs, sizes, offset)
}

// ReadPacket 从TUN设备读取单个数据包，支持数据包缓存
func (t *TUNDevice) ReadPacket(packet []byte, offset int) (int, error) {
	t.mu.Lock()

	// 懒初始化（只在首次调用时进行）
	if t.packetBufs == nil {
		t.mu.Unlock() // 释放锁进行初始化

		batchSize := t.device.BatchSize()
		if batchSize <= 0 {
			batchSize = 32
		}

		packetBufs := make([][]byte, batchSize)
		for i := range packetBufs {
			packetBufs[i] = make([]byte, 2048) // 固定分配
		}
		sizes := make([]int, batchSize)

		t.mu.Lock()
		// 二次检查防止并发初始化
		if t.packetBufs == nil {
			t.batchSize = batchSize
			t.maxPacketSize = 2048
			t.packetBufs = packetBufs
			t.sizes = sizes
		}
	}

	// 如果缓存为空，填充缓存(保持锁定状态进行批量读取)
	if t.count == 0 {
		// 使用非阻塞模式读取(性能关键区域不应阻塞)
		n, err := t.device.Read(t.packetBufs, t.sizes, 0)
		if err != nil {
			t.mu.Unlock()
			return 0, err
		}

		if n == 0 {
			t.mu.Unlock()
			return 0, nil
		}

		t.head = 0
		t.count = n
	}

	// 获取并复制数据包
	size := t.sizes[t.head]
	if len(packet[offset:]) < size {
		t.mu.Unlock()
		return 0, errors.New("buffer too small for packet")
	}

	// 使用更高效的copy，编译器可能会优化为memcpy
	copy(packet[offset:], t.packetBufs[t.head][:size])

	t.head = (t.head + 1) % t.batchSize
	t.count--

	t.mu.Unlock()
	return size, nil
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
