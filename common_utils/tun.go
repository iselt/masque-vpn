package common_utils

import (
	"net/netip"
	"sync"
)

// 为TUN设备读写操作预分配的缓冲区结构
type tunBuffers struct {
	bufs  [][]byte // 用于保存数据包的切片数组
	sizes []int    // 用于保存包大小的整数数组
}

// 创建对象池，重用tunBuffers结构
var tunBuffersPool = sync.Pool{
	New: func() interface{} {
		return &tunBuffers{
			bufs:  make([][]byte, 1),
			sizes: make([]int, 1),
		}
	},
}

// Read 从TUN设备读取数据，使用原生接口直接传递参数
func (t *TUNDevice) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	return t.device.Read(bufs, sizes, offset)
}

// ReadPacket 从TUN设备读取单个数据包，优化版本使用对象池减少内存分配
func (t *TUNDevice) ReadPacket(packet []byte, offset int) (int, error) {
	// 从池中获取预分配的结构
	tb := tunBuffersPool.Get().(*tunBuffers)

	// 设置packet引用
	tb.bufs[0] = packet
	tb.sizes[0] = 0

	// 使用预分配的切片调用底层接口
	n, err := t.device.Read(tb.bufs, tb.sizes, offset)

	// 获取结果
	var size int
	if n > 0 {
		size = tb.sizes[0]
	}

	// 清除引用以避免内存泄漏
	tb.bufs[0] = nil

	// 归还到池中
	tunBuffersPool.Put(tb)

	if err != nil {
		return 0, err
	}

	if n == 0 {
		return 0, nil
	}

	return size, nil
}

// Write 向TUN设备写入数据，使用原生接口直接传递参数
func (t *TUNDevice) Write(bufs [][]byte, offset int) (int, error) {
	return t.device.Write(bufs, offset)
}

// WritePacket 向TUN设备写入单个数据包，优化版本使用对象池减少内存分配
func (t *TUNDevice) WritePacket(packet []byte, offset int) (int, error) {
	// 从池中获取预分配的结构
	tb := tunBuffersPool.Get().(*tunBuffers)

	// 设置packet引用
	tb.bufs[0] = packet

	// 使用预分配的切片调用底层接口
	n, err := t.device.Write(tb.bufs, offset)

	// 清除引用以避免内存泄漏
	tb.bufs[0] = nil

	// 归还到池中
	tunBuffersPool.Put(tb)

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
