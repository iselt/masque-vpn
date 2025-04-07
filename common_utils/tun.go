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

	// 如果缓存为空，则重新填充缓存
	if t.count == 0 {
		t.mu.Unlock() // 在可能阻塞的调用前释放锁

		// 懒初始化
		if t.packetBufs == nil {
			t.batchSize = t.device.BatchSize()
			if t.batchSize <= 0 {
				t.batchSize = 32 // 默认批量大小
			}
			t.maxPacketSize = 2048 // 可以根据实际情况调整，标准IPv4包最大1500字节左右
			t.packetBufs = make([][]byte, t.batchSize)
			for i := range t.packetBufs {
				t.packetBufs[i] = make([]byte, t.maxPacketSize)
			}
			t.sizes = make([]int, t.batchSize)
			t.head = 0
			t.count = 0
		}

		// 批量读取数据包
		n, err := t.device.Read(t.packetBufs, t.sizes, 0) // 使用0偏移，因为我们使用自己的缓冲区

		t.mu.Lock() // 重新获取锁
		if err != nil {
			t.mu.Unlock()
			return 0, err
		}

		if n == 0 {
			t.mu.Unlock()
			return 0, nil // 没有读取到数据包
		}

		// 更新缓存状态
		t.head = 0
		t.count = n
	}

	// 从缓存中取出一个数据包
	size := t.sizes[t.head]

	// 确保用户提供的缓冲区足够大
	if len(packet[offset:]) < size {
		t.mu.Unlock()
		return 0, errors.New("buffer too small for packet")
	}

	// 复制数据包到用户提供的缓冲区
	copy(packet[offset:], t.packetBufs[t.head][:size])

	// 更新队列状态
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
