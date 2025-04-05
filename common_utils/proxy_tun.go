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

// 定义缓冲区大小
const (
	BufferSize = 2048 // 标准MTU大小
)

// ProxyFromTunToVPN 从TUN设备读取数据包并发送到VPN连接
func ProxyFromTunToVPN(dev *TUNDevice, ipconn *connectip.Conn, errChan chan<- error) {
	// 创建单个缓冲区
	buf := make([]byte, BufferSize)

	for {
		// 单个读取数据包
		n, err := dev.Read(buf)
		if err != nil {
			if errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
				log.Println("TUN device closed, stopping Tun->VPN proxy.")
				errChan <- nil
				return
			}
			errChan <- fmt.Errorf("failed to read from TUN device %s: %w", dev.Name(), err)
			return
		}

		if n == 0 {
			continue
		}

		// 写入VPN连接
		icmp, err := ipconn.WritePacket(buf[:n])
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

			if _, err := dev.Write(icmp); err != nil {
				log.Printf("Warning: Unable to write ICMP packet to TUN device %s: %v", dev.Name(), err)
			}
		}
	}
}

// ProxyFromVPNToTun 从VPN连接读取数据包并写入TUN设备
func ProxyFromVPNToTun(dev *TUNDevice, ipconn *connectip.Conn, errChan chan<- error) {
	// 创建读取缓冲区
	buf := make([]byte, BufferSize)

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

		// 写入TUN设备
		if _, err := dev.Write(buf[:n]); err != nil {
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
