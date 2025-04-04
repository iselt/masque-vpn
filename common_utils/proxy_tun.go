package common_utils

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	connectip "github.com/quic-go/connect-ip-go"
	"github.com/quic-go/quic-go"
)

// ProxyFromTunToVPN 从TUN设备读取数据包并发送到VPN连接
func ProxyFromTunToVPN(dev *TUNDevice, ipconn *connectip.Conn, errChan chan<- error) {
	// 获取设备推荐的批处理大小
	batchSize := dev.BatchSize()
	log.Printf("Recommended batch size for TUN device %s: %d", dev.Name(), batchSize)
	if batchSize <= 0 {
		batchSize = 1 // 如果设备没有指定批处理大小，默认为1
	}

	// 预分配缓冲区和大小记录数组
	bufs := make([][]byte, batchSize)
	sizes := make([]int, batchSize)
	for i := 0; i < batchSize; i++ {
		bufs[i] = make([]byte, 2048) // 假设MTU不超过2048
	}

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

			// log.Printf("Writing packet (%d bytes) from TUN device %s to connect-ip connection", sizes[i], dev.Name())

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
				icmpPacket := make([]byte, len(icmp))
				copy(icmpPacket, icmp)
				if _, err := dev.Write([][]byte{icmpPacket}, 0); err != nil {
					log.Printf("Warning: Unable to write ICMP packet to TUN device %s: %v", dev.Name(), err)
				}
			}
		}
	}
}

// ProxyFromVPNToTun 从VPN连接读取数据包并写入TUN设备
func ProxyFromVPNToTun(dev *TUNDevice, ipconn *connectip.Conn, errChan chan<- error) {
	// 预分配读取缓冲区
	buf := make([]byte, 2048)

	// 虚拟网络头部大小（对于支持vnetHdr的设备）
	const virtioNetHdrLen = 10

	for {
		// 从VPN连接读取数据包
		n, err := ipconn.ReadPacket(buf)
		if err != nil {
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
			continue
		}

		// log.Printf("Writing packet (%d bytes) from connect-ip to TUN device %s", n, dev.Name())

		// 创建数据包副本并为可能的virtio头部预留空间
		packet := make([]byte, virtioNetHdrLen+n)
		// 将实际数据拷贝到预留空间之后
		copy(packet[virtioNetHdrLen:], buf[:n])

		// 使用virtioNetHdrLen作为offset，这样即使设备内部减去这个值，
		// 最终使用的offset也不会变为负数
		if _, err := dev.Write([][]byte{packet}, virtioNetHdrLen); err != nil {
			if errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
				log.Println("TUN device closed, stopping VPN->Tun proxy.")
				errChan <- nil
			} else {
				errChan <- fmt.Errorf("failed to write packet to TUN device %s: %w", dev.Name(), err)
			}
			return
		}
	}
}
