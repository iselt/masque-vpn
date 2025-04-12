package main

import (
	"flag"
	"fmt"
	"log"
	"net/netip"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/iselt/masque-vpn/common_utils"
)

const (
	DefaultTestDurationSec = 10
	DefaultPacketSize      = 1400 // 典型MTU以下的数据包大小
	DefaultBatchSize       = 32   // 默认批量操作大小
)

func main() {
	// 命令行参数
	durationSec := flag.Int("duration", DefaultTestDurationSec, "测试持续时间(秒)")
	packetSize := flag.Int("size", DefaultPacketSize, "数据包大小(字节)")
	batchMode := flag.Bool("batch", false, "是否使用批量模式测试")
	batchSize := flag.Int("batch-size", DefaultBatchSize, "批量大小")
	writeTest := flag.Bool("write", true, "测试写入性能")
	readTest := flag.Bool("read", true, "测试读取性能")
	mtu := flag.Int("mtu", 1500, "TUN设备MTU大小")
	flag.Parse()

	// 创建TUN设备
	prefix, err := netip.ParsePrefix("192.168.100.1/24")
	if err != nil {
		log.Fatalf("解析IP前缀失败: %v", err)
	}

	tunDev, err := common_utils.CreateTunDevice("tunspeed", prefix, *mtu)
	if err != nil {
		log.Fatalf("创建TUN设备失败: %v", err)
	}
	defer tunDev.Close()

	fmt.Printf("已创建TUN设备 %s 进行性能测试\n", tunDev.Name())

	// 设置信号处理，确保优雅退出
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// 测试写入性能
	if *writeTest {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if *batchMode {
				testBatchWrite(tunDev, *packetSize, *batchSize, *durationSec)
			} else {
				testWrite(tunDev, *packetSize, *durationSec)
			}
		}()
	}

	// 测试读取性能
	if *readTest {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 稍微延迟启动读取测试，确保已经有数据包可读
			time.Sleep(100 * time.Millisecond)
			if *batchMode {
				testBatchRead(tunDev, *batchSize, *durationSec)
			} else {
				testRead(tunDev, *durationSec)
			}
		}()
	}

	// 等待信号或测试完成
	go func() {
		<-signalChan
		fmt.Println("\n接收到终止信号，正在停止测试...")
		tunDev.Close()
	}()

	wg.Wait()
}

// 测试单包写入性能
func testWrite(tunDev *common_utils.TUNDevice, packetSize, durationSec int) {
	fmt.Printf("测试单包写入性能 (包大小: %d 字节, 持续时间: %d 秒)...\n", packetSize, durationSec)

	// 创建测试IPv4数据包
	packet := createTestPacket(packetSize)

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	startAlloc := stats.TotalAlloc

	startTime := time.Now()
	deadline := startTime.Add(time.Duration(durationSec) * time.Second)

	var packetsWritten uint64
	var bytesWritten uint64

	for time.Now().Before(deadline) {
		n, err := tunDev.WritePacket(packet, 0)
		if err != nil {
			log.Printf("写入错误: %v", err)
			break
		}

		packetsWritten++
		bytesWritten += uint64(n)
	}

	runtime.ReadMemStats(&stats)
	endAlloc := stats.TotalAlloc
	memoryAllocated := endAlloc - startAlloc

	reportPerformance("写入", startTime, packetsWritten, bytesWritten, memoryAllocated)
}

// 测试批量写入性能
func testBatchWrite(tunDev *common_utils.TUNDevice, packetSize, batchSize, durationSec int) {
	fmt.Printf("测试批量写入性能 (包大小: %d 字节, 批量大小: %d, 持续时间: %d 秒)...\n",
		packetSize, batchSize, durationSec)

	// 创建测试数据包批次
	bufs := make([][]byte, batchSize)
	for i := range bufs {
		bufs[i] = createTestPacket(packetSize)
	}

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	startAlloc := stats.TotalAlloc

	startTime := time.Now()
	deadline := startTime.Add(time.Duration(durationSec) * time.Second)

	var batchesWritten uint64
	var packetsWritten uint64
	var bytesWritten uint64

	for time.Now().Before(deadline) {
		n, err := tunDev.Write(bufs, 0)
		if err != nil {
			log.Printf("批量写入错误: %v", err)
			break
		}

		batchesWritten++
		packetsWritten += uint64(n)
		bytesWritten += uint64(n * packetSize)
	}

	runtime.ReadMemStats(&stats)
	endAlloc := stats.TotalAlloc
	memoryAllocated := endAlloc - startAlloc

	fmt.Printf("批量写入性能:\n")
	fmt.Printf("  总批次数: %d\n", batchesWritten)
	fmt.Printf("  总包数: %d\n", packetsWritten)
	fmt.Printf("  总字节数: %d (%.2f MB)\n", bytesWritten, float64(bytesWritten)/(1024*1024))
	fmt.Printf("  批次/秒: %.2f\n", float64(batchesWritten)/time.Since(startTime).Seconds())
	fmt.Printf("  包/秒: %.2f\n", float64(packetsWritten)/time.Since(startTime).Seconds())
	fmt.Printf("  吞吐量: %.2f MB/秒\n", float64(bytesWritten)/time.Since(startTime).Seconds()/(1024*1024))
	fmt.Printf("  分配内存: %.2f MB\n", float64(memoryAllocated)/(1024*1024))
}

// 测试单包读取性能
func testRead(tunDev *common_utils.TUNDevice, durationSec int) {
	fmt.Printf("测试单包读取性能 (持续时间: %d 秒)...\n", durationSec)

	// 创建读取缓冲区
	buffer := make([]byte, 2048)

	// 启动辅助写入协程，确保有数据可读
	var wg sync.WaitGroup
	wg.Add(1)
	stopWrite := make(chan struct{})
	go func() {
		defer wg.Done()
		writePacketForReadTest(tunDev, stopWrite)
	}()

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	startAlloc := stats.TotalAlloc

	startTime := time.Now()
	deadline := startTime.Add(time.Duration(durationSec) * time.Second)

	var packetsRead uint64
	var bytesRead uint64

	for time.Now().Before(deadline) {
		n, err := tunDev.ReadPacket(buffer, 0)
		if err != nil {
			log.Printf("读取错误: %v", err)
			break
		}

		if n > 0 {
			packetsRead++
			bytesRead += uint64(n)
		}
	}

	runtime.ReadMemStats(&stats)
	endAlloc := stats.TotalAlloc
	memoryAllocated := endAlloc - startAlloc

	// 停止写入协程
	close(stopWrite)
	wg.Wait()

	reportPerformance("读取", startTime, packetsRead, bytesRead, memoryAllocated)
}

// 测试批量读取性能
func testBatchRead(tunDev *common_utils.TUNDevice, batchSize, durationSec int) {
	fmt.Printf("测试批量读取性能 (批量大小: %d, 持续时间: %d 秒)...\n", batchSize, durationSec)

	// 创建批量读取缓冲区
	bufs := make([][]byte, batchSize)
	sizes := make([]int, batchSize)
	for i := range bufs {
		bufs[i] = make([]byte, 2048)
	}

	// 启动辅助写入协程，确保有数据可读
	var wg sync.WaitGroup
	wg.Add(1)
	stopWrite := make(chan struct{})
	go func() {
		defer wg.Done()
		writeBatchForReadTest(tunDev, batchSize, stopWrite)
	}()

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	startAlloc := stats.TotalAlloc

	startTime := time.Now()
	deadline := startTime.Add(time.Duration(durationSec) * time.Second)

	var batchesRead uint64
	var packetsRead uint64
	var bytesRead uint64

	for time.Now().Before(deadline) {
		n, err := tunDev.Read(bufs, sizes, 0)
		if err != nil {
			log.Printf("批量读取错误: %v", err)
			break
		}

		if n > 0 {
			batchesRead++
			packetsRead += uint64(n)

			for i := 0; i < n; i++ {
				bytesRead += uint64(sizes[i])
			}
		}
	}

	runtime.ReadMemStats(&stats)
	endAlloc := stats.TotalAlloc
	memoryAllocated := endAlloc - startAlloc

	// 停止写入协程
	close(stopWrite)
	wg.Wait()

	fmt.Printf("批量读取性能:\n")
	fmt.Printf("  总批次数: %d\n", batchesRead)
	fmt.Printf("  总包数: %d\n", packetsRead)
	fmt.Printf("  总字节数: %d (%.2f MB)\n", bytesRead, float64(bytesRead)/(1024*1024))
	fmt.Printf("  批次/秒: %.2f\n", float64(batchesRead)/time.Since(startTime).Seconds())
	fmt.Printf("  包/秒: %.2f\n", float64(packetsRead)/time.Since(startTime).Seconds())
	fmt.Printf("  吞吐量: %.2f MB/秒\n", float64(bytesRead)/time.Since(startTime).Seconds()/(1024*1024))
	fmt.Printf("  分配内存: %.2f MB\n", float64(memoryAllocated)/(1024*1024))
}

// 创建测试用的IPv4数据包
func createTestPacket(size int) []byte {
	packet := make([]byte, size)
	// 设置IPv4版本和头部长度
	packet[0] = 0x45 // IPv4, 头部长度20字节
	// 设置协议为TCP(6)
	packet[9] = 6
	// 设置源IP和目标IP
	copy(packet[12:16], []byte{192, 168, 100, 1})
	copy(packet[16:20], []byte{192, 168, 100, 2})
	return packet
}

// 用于读测试的辅助写入函数
func writePacketForReadTest(tunDev *common_utils.TUNDevice, stop chan struct{}) {
	packet := createTestPacket(1400)
	for {
		select {
		case <-stop:
			return
		default:
			tunDev.WritePacket(packet, 0)
			time.Sleep(time.Microsecond)
		}
	}
}

// 用于批量读测试的辅助写入函数
func writeBatchForReadTest(tunDev *common_utils.TUNDevice, batchSize int, stop chan struct{}) {
	bufs := make([][]byte, batchSize)
	for i := range bufs {
		bufs[i] = createTestPacket(1400)
	}

	for {
		select {
		case <-stop:
			return
		default:
			tunDev.Write(bufs, 0)
			time.Sleep(time.Microsecond)
		}
	}
}

// 报告性能结果
func reportPerformance(testType string, startTime time.Time, packets, bytes, memAlloc uint64) {
	duration := time.Since(startTime)
	packetRate := float64(packets) / duration.Seconds()
	bytesRate := float64(bytes) / duration.Seconds()

	fmt.Printf("%s性能:\n", testType)
	fmt.Printf("  总包数: %d\n", packets)
	fmt.Printf("  总字节数: %d (%.2f MB)\n", bytes, float64(bytes)/(1024*1024))
	fmt.Printf("  包/秒: %.2f\n", packetRate)
	fmt.Printf("  吞吐量: %.2f MB/秒\n", bytesRate/(1024*1024))
	fmt.Printf("  分配内存: %.2f MB\n", float64(memAlloc)/(1024*1024))
}
