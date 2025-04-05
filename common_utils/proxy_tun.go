package common_utils

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"

	connectip "github.com/quic-go/connect-ip-go"
	"github.com/quic-go/quic-go"
)

// 定义全局缓冲区池
var (
	// 用于VPN到TUN的包传输
	packetBufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 2048+10) // 预留virtioNetHdr的10字节，加上2048的MTU大小
			return &buf
		},
	}

	// 用于TUN到VPN的批量读取缓冲区
	tunReadBufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 2048) // 最大MTU大小
			return &buf
		},
	}
)

// ProxyFromTunToVPN 从TUN设备读取数据包并发送到VPN连接
func ProxyFromTunToVPN(dev *TUNDevice, ipconn *connectip.Conn, errChan chan<- error) {
	// 获取设备推荐的批处理大小
	batchSize := dev.BatchSize()
	log.Printf("Recommended batch size for TUN device %s: %d", dev.Name(), batchSize)
	if batchSize <= 0 {
		batchSize = 32 // 使用合理的默认批处理大小
	}

	// 预分配缓冲区和大小记录数组
	bufs := make([][]byte, batchSize)
	sizes := make([]int, batchSize)
	bufPtrs := make([]*[]byte, batchSize)

	// 从池中获取缓冲区
	for i := 0; i < batchSize; i++ {
		bufPtrs[i] = tunReadBufferPool.Get().(*[]byte)
		bufs[i] = *bufPtrs[i]
	}

	// 确保函数退出时归还缓冲区
	defer func() {
		for i := 0; i < batchSize; i++ {
			if bufPtrs[i] != nil {
				tunReadBufferPool.Put(bufPtrs[i])
			}
		}
	}()

	for {
		// 批量读取数据包
		n, err := dev.Read(bufs, sizes, 0)
		if err != nil {
			if errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
				log.Println("TUN device closed, stopping Tun->VPN proxy.")
				errChan <- nil
				return
			}
			errChan <- fmt.Errorf("failed to read from TUN device %s: %w", dev.Name(), err)
			return
		}

		// 处理读取到的每个数据包
		for i := 0; i < n; i++ {
			if sizes[i] == 0 {
				continue
			}

			// 写入VPN连接
			icmp, err := ipconn.WritePacket(bufs[i][:sizes[i]])
			if err != nil {
				var netErr *net.OpError
				var qErr *quic.ApplicationError
				if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.As(err, &netErr) || errors.As(err, &qErr) {
					log.Println("Connection closed, stopping Tun->VPN proxy.")
					errChan <- nil
				} else {
					errChan <- fmt.Errorf("failed to write packet to connect-ip connection: %w", err)
				}
				return
			}

			// 处理ICMP响应
			if len(icmp) > 0 {
				log.Printf("Writing ICMP packet (%d bytes) from connect-ip back to TUN device %s", len(icmp), dev.Name())
				// 从池中获取ICMP响应缓冲区
				icmpBufPtr := packetBufferPool.Get().(*[]byte)
				icmpBuf := *icmpBufPtr
				copy(icmpBuf[:len(icmp)], icmp)

				if _, err := dev.Write([][]byte{icmpBuf[:len(icmp)]}, 0); err != nil {
					log.Printf("Warning: Unable to write ICMP packet to TUN device %s: %v", dev.Name(), err)
				}

				// 使用完毕，归还缓冲区
				packetBufferPool.Put(icmpBufPtr)
			}
		}
	}
}

// ProxyFromVPNToTun 从VPN连接读取数据包并写入TUN设备
// 使用简化但可靠的实现，每次处理一个数据包
func ProxyFromVPNToTun(dev *TUNDevice, ipconn *connectip.Conn, errChan chan<- error) {
	// 虚拟网络头部大小
	const virtioNetHdrLen = 10

	for {
		// 从池中获取读取缓冲区
		readBufPtr := packetBufferPool.Get().(*[]byte)
		readBuf := *readBufPtr

		// 从VPN连接读取数据包
		n, err := ipconn.ReadPacket(readBuf[virtioNetHdrLen:])
		if err != nil {
			packetBufferPool.Put(readBufPtr) // 确保归还缓冲区

			var netErr *net.OpError
			var qErr *quic.ApplicationError
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.As(err, &netErr) || errors.As(err, &qErr) {
				log.Println("Connection closed, stopping VPN->Tun proxy.")
				errChan <- nil
			} else {
				errChan <- fmt.Errorf("failed to read packet from connect-ip connection: %w", err)
			}
			return
		}

		if n == 0 {
			packetBufferPool.Put(readBufPtr) // 归还缓冲区
			continue
		}

		// 准备写入TUN设备的数据 - 包含virtioNetHdr
		packet := readBuf[:virtioNetHdrLen+n]

		// 写入TUN设备
		if _, err := dev.Write([][]byte{packet}, virtioNetHdrLen); err != nil {
			packetBufferPool.Put(readBufPtr) // 错误情况下也要归还缓冲区

			if errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
				log.Println("TUN device closed, stopping VPN->Tun proxy.")
				errChan <- nil
			} else {
				errChan <- fmt.Errorf("failed to write packet to TUN device %s: %w", dev.Name(), err)
			}
			return
		}

		// 使用完毕，归还缓冲区
		packetBufferPool.Put(readBufPtr)
	}
}
