package common

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"

	connectip "github.com/iselt/connect-ip-go"
	"github.com/quic-go/quic-go"
	"golang.zx2c4.com/wireguard/tun"
)

// 定义缓冲区大小
const (
	BufferSize      = 2048 // 标准MTU大小
	VirtioNetHdrLen = 10   // virtio-net 头部长度
)

// 为TUN->VPN方向创建缓冲区池
var tunToVPNBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, BufferSize)
	},
}

// 为VPN->TUN方向创建缓冲区池（包含virtio-net头部）
var vpnToTunBufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, BufferSize+VirtioNetHdrLen)
		// 预先清零头部区域
		for i := 0; i < VirtioNetHdrLen; i++ {
			buf[i] = 0
		}
		return buf
	},
}

// isNetworkClosed 判断错误是否表示网络连接已关闭
func isNetworkClosed(err error) bool {
	var netErr *net.OpError
	var qErr *quic.ApplicationError

	return errors.Is(err, io.EOF) ||
		errors.Is(err, net.ErrClosed) ||
		errors.As(err, &netErr) ||
		errors.As(err, &qErr)
}

// ProxyFromTunToVPN 从TUN设备读取数据包并发送到VPN连接
// 优化版本：直接使用批量读取
func ProxyFromTunToVPN(dev *TUNDevice, ipconn *connectip.Conn, errChan chan<- error) {
	// 确定批量大小
	batchSize := dev.BatchSize()
	if batchSize <= 0 {
		batchSize = 32 // 默认批量大小
	}

	// 预先分配批量读取的缓冲区
	packetBufs := make([][]byte, batchSize)
	sizes := make([]int, batchSize)
	for i := range packetBufs {
		packetBufs[i] = make([]byte, BufferSize)
	}

	for {
		// 直接从TUN设备批量读取数据包
		n, err := dev.Read(packetBufs, sizes, 0)

		if err != nil {
			if errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
				log.Println("TUN device closed, stopping Tun->VPN proxy.")
				errChan <- nil
				return
			}

			if errors.Is(err, tun.ErrTooManySegments) {
				log.Println("Warning: Too many segments in TUN device read, continuing...")
				continue
			}

			errChan <- fmt.Errorf("failed to read batch from TUN device %s: %w", dev.Name(), err)
			return
		}

		if n == 0 {
			continue // 这批次没有数据包
		}

		// 处理批次中的每个数据包
		for i := 0; i < n; i++ {
			packetData := packetBufs[i][:sizes[i]]

			// 发送数据包到VPN连接
			icmp, writeErr := ipconn.WritePacket(packetData)

			if writeErr != nil {
				if isNetworkClosed(writeErr) {
					log.Println("Connection closed, stopping Tun->VPN proxy.")
					errChan <- nil
				} else {
					errChan <- fmt.Errorf("failed to write to connect-ip connection: %w", writeErr)
				}
				return
			}

			// 处理ICMP响应（如果有）
			if len(icmp) > 0 {
				if _, err := dev.WritePacket(icmp, 0); err != nil {
					if !errors.Is(err, os.ErrClosed) && !errors.Is(err, net.ErrClosed) {
						log.Printf("Warning: Unable to write ICMP packet to TUN device %s: %v", dev.Name(), err)
					}
				}
			}
		}
	}
}

// ProxyFromVPNToTun 从VPN连接读取数据包并写入TUN设备
func ProxyFromVPNToTun(dev *TUNDevice, ipconn *connectip.Conn, errChan chan<- error) {
	for {
		// 从池中获取预先准备好virtio头的缓冲区
		buf := vpnToTunBufferPool.Get().([]byte)

		// 直接读取到缓冲区的offset位置，避免后续的复制操作
		n, err := ipconn.ReadPacket(buf[VirtioNetHdrLen:])

		if err != nil {
			vpnToTunBufferPool.Put(buf) // 归还缓冲区

			if isNetworkClosed(err) {
				log.Println("Connection closed, stopping VPN->Tun proxy.")
				errChan <- nil
			} else {
				errChan <- fmt.Errorf("failed to read from connect-ip connection: %w", err)
			}
			return
		}

		if n == 0 {
			vpnToTunBufferPool.Put(buf) // 归还缓冲区
			continue
		}

		// 写入单个数据包到TUN设备
		if _, err := dev.WritePacket(buf[:n+VirtioNetHdrLen], VirtioNetHdrLen); err != nil {
			vpnToTunBufferPool.Put(buf) // 归还缓冲区

			if errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
				log.Println("TUN device closed, stopping VPN->Tun proxy.")
				errChan <- nil
			} else {
				errChan <- fmt.Errorf("failed to write packet to TUN device %s: %w", dev.Name(), err)
			}
			return
		}

		// 归还缓冲区
		vpnToTunBufferPool.Put(buf)
	}
}
