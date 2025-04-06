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
func ProxyFromTunToVPN(dev *TUNDevice, ipconn *connectip.Conn, errChan chan<- error) {
	for {
		// 从池中获取缓冲区
		buf := tunToVPNBufferPool.Get().([]byte)

		// 从TUN设备读取单个数据包
		n, err := dev.ReadPacket(buf, 0)

		if err != nil {
			tunToVPNBufferPool.Put(buf) // 归还缓冲区

			if errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
				log.Println("TUN device closed, stopping Tun->VPN proxy.")
				errChan <- nil
				return
			}

			if errors.Is(err, tun.ErrTooManySegments) {
				log.Println("Warning: Too many segments in TUN device read, continuing...")
				continue // 继续循环，不退出
			}

			errChan <- fmt.Errorf("failed to read from TUN device %s: %w", dev.Name(), err)
			return
		}

		if n == 0 {
			tunToVPNBufferPool.Put(buf) // 归还缓冲区
			continue
		}

		// 发送数据包到VPN连接
		icmp, err := ipconn.WritePacket(buf[:n])

		// 先归还缓冲区，避免后续处理忘记归还
		tunToVPNBufferPool.Put(buf)

		if err != nil {
			if isNetworkClosed(err) {
				log.Println("Connection closed, stopping Tun->VPN proxy.")
				errChan <- nil
			} else {
				errChan <- fmt.Errorf("failed to write to connect-ip connection: %w", err)
			}
			return
		}

		// 如果有ICMP响应，写回TUN设备
		if len(icmp) > 0 {
			if _, err := dev.WritePacket(icmp, 0); err != nil {
				if !errors.Is(err, os.ErrClosed) && !errors.Is(err, net.ErrClosed) {
					log.Printf("Warning: Unable to write ICMP packet to TUN device %s: %v", dev.Name(), err)
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
