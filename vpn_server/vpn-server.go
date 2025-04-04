package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/iselt/masque-vpn/common_utils" // Import local module

	"github.com/BurntSushi/toml"
	connectip "github.com/quic-go/connect-ip-go"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/yosida95/uritemplate/v3"
)

// ServerConfig holds configuration values loaded from TOML
type ServerConfig struct {
	ListenAddr      string   `toml:"listen_addr"`
	CertFile        string   `toml:"cert_file"`
	KeyFile         string   `toml:"key_file"`
	AssignCIDR      string   `toml:"assign_cidr"` // 整个 VPN 网段配置
	AdvertiseRoutes []string `toml:"advertise_routes"`
	TunName         string   `toml:"tun_name"` // 可选的 TUN 设备名称
	LogLevel        string   `toml:"log_level"`
	ServerName      string   `toml:"server_name"`
}

var serverConfig ServerConfig
var serverSendSocket int = -1
var serverRecvSocket int = -1
var interfaceAddr netip.Addr

func main() {
	// --- 配置加载 ---
	configFile := "config.server.toml"
	if _, err := toml.DecodeFile(configFile, &serverConfig); err != nil {
		log.Fatalf("Error loading config file %s: %v", configFile, err)
	}

	// --- 基础验证 ---
	if serverConfig.ListenAddr == "" || serverConfig.CertFile == "" || serverConfig.KeyFile == "" ||
		serverConfig.AssignCIDR == "" || serverConfig.ServerName == "" {
		log.Fatal("Missing required configuration values in config.server.toml")
	}

	// --- 创建 IP 分配器 ---
	networkInfo, err := NewNetworkInfo(serverConfig.AssignCIDR)
	if err != nil {
		log.Fatalf("Failed to create IP allocator: %v", err)
	}

	log.Printf("Starting VPN Server...")
	log.Printf("Listen Address: %s", serverConfig.ListenAddr)
	log.Printf("VPN Network: %s", networkInfo.GetPrefix())
	log.Printf("Gateway IP: %s", networkInfo.GetGateway())
	log.Printf("Advertised Routes: %v", serverConfig.AdvertiseRoutes)

	// --- 创建 TUN 设备 ---
	tunDev, err := common_utils.CreateTunDevice(serverConfig.TunName, networkInfo.GetGateway())
	if err != nil {
		log.Fatalf("Failed to create TUN device: %v", err)
	}
	defer tunDev.Close()

	// --- 准备路由信息 ---
	var routesToAdvertise []connectip.IPRoute
	for _, routeStr := range serverConfig.AdvertiseRoutes {
		prefix, err := netip.ParsePrefix(routeStr)
		if err != nil {
			log.Fatalf("Invalid route in advertise_routes '%s': %v", routeStr, err)
		}
		routesToAdvertise = append(routesToAdvertise, connectip.IPRoute{
			StartIP:    prefix.Addr(),
			EndIP:      common_utils.LastIP(prefix),
			IPProtocol: 0, // 0 表示任何协议
		})
	}

	// --- 创建原始套接字 ---
	// 不再使用物理网卡，而是使用 TUN 设备
	// 为读写 TUN 设备设置缓冲区
	serverRecvSocket = -1 // 不再需要
	serverSendSocket = -1 // 不再需要

	// --- TLS 配置 ---
	cert, err := tls.LoadX509KeyPair(serverConfig.CertFile, serverConfig.KeyFile)
	if err != nil {
		log.Fatalf("Failed to load TLS certificate/key: %v", err)
	}
	tlsConfig := http3.ConfigureTLSConfig(&tls.Config{
		Certificates: []tls.Certificate{cert},
	})

	// --- QUIC 监听器 ---
	listenNetAddr, err := net.ResolveUDPAddr("udp", serverConfig.ListenAddr)
	if err != nil {
		log.Fatalf("Failed to resolve listen address %s: %v", serverConfig.ListenAddr, err)
	}
	udpConn, err := net.ListenUDP("udp", listenNetAddr)
	if err != nil {
		log.Fatalf("Failed to listen on UDP %s: %v", serverConfig.ListenAddr, err)
	}
	defer udpConn.Close()

	ln, err := quic.ListenEarly(udpConn, tlsConfig, &quic.Config{EnableDatagrams: true})
	if err != nil {
		log.Fatalf("Failed to create QUIC listener: %v", err)
	}
	defer ln.Close()
	log.Printf("QUIC Listener started on %s", udpConn.LocalAddr())

	// --- CONNECT-IP 代理和 HTTP 处理程序 ---
	p := connectip.Proxy{}
	// Use the configured server name and port for the template
	serverHost, serverPortStr, _ := net.SplitHostPort(serverConfig.ListenAddr)
	if serverHost == "0.0.0.0" || serverHost == "[::]" || serverHost == "" {
		serverHost = serverConfig.ServerName // Use configured name if listening on wildcard
	}
	serverPort, _ := strconv.Atoi(serverPortStr)
	template := uritemplate.MustNew(fmt.Sprintf("https://%s:%d/vpn", serverHost, serverPort))

	mux := http.NewServeMux()
	mux.HandleFunc("/vpn", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Incoming VPN request from %s", r.RemoteAddr)
		req, err := connectip.ParseRequest(r, template)
		if err != nil {
			log.Printf("Failed to parse connect-ip request: %v", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// 建立服务器端 connect-ip 连接
		conn, err := p.Proxy(w, req)
		if err != nil {
			log.Printf("Failed to establish connect-ip proxy connection: %v", err)
			return
		}

		// 为客户端生成唯一 ID (使用其远程地址)
		clientID := r.RemoteAddr
		log.Printf("CONNECT-IP connection established for %s", clientID)

		// 处理客户端连接
		go handleClientConnection(conn, clientID, tunDev, networkInfo, routesToAdvertise)
	})

	// --- HTTP/3 Server ---
	h3Server := http3.Server{
		Handler:         mux,
		EnableDatagrams: true,
		QUICConfig:      &quic.Config{EnableDatagrams: true},
	}

	// --- Graceful Shutdown Handling ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("Starting HTTP/3 server...")
		if err := h3Server.ServeListener(ln); err != nil && err != http.ErrServerClosed && err != quic.ErrServerClosed {
			log.Printf("HTTP/3 server error: %v", err)
		}
		log.Println("HTTP/3 server stopped.")
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutdown signal received...")

	// Initiate graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // 10 seconds timeout
	defer cancel()

	if err := h3Server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during HTTP/3 server graceful shutdown: %v", err)
	} else {
		log.Println("HTTP/3 server shutdown gracefully.")
	}

	// Wait for the server goroutine to finish
	wg.Wait()
	log.Println("Server exited.")
}

// 更新客户端连接处理函数
func handleClientConnection(conn *connectip.Conn, clientID string,
	tunDev *common_utils.TUNDevice, networkInfo *NetworkInfo, routes []connectip.IPRoute) {
	defer conn.Close()

	log.Printf("Handling connection for client %s", clientID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// --- 为客户端分配 IP 地址 ---
	// 分配整个网络前缀 (用于路由信息)
	networkPrefix := networkInfo.GetPrefix()

	// 向客户端分配网络
	if err := conn.AssignAddresses(ctx, []netip.Prefix{networkPrefix}); err != nil {
		log.Printf("Error assigning addresses to client %s: %v", clientID, err)
		return
	}
	log.Printf("Assigned network %s to client %s", networkPrefix, clientID)

	// --- 向客户端广播路由 ---
	if err := conn.AdvertiseRoute(ctx, routes); err != nil {
		log.Printf("Error advertising routes to client %s: %v", clientID, err)
		return
	}
	log.Printf("Advertised %d routes to client %s", len(routes), clientID)

	// --- 代理流量 ---
	errChan := make(chan error, 2)
	var wg sync.WaitGroup

	wg.Add(2)
	// 从 VPN 连接读取 -> 写入 TUN 设备
	go func() {
		defer wg.Done()
		common_utils.ProxyFromVPNToTun(tunDev, conn, errChan)
	}()

	// 从 TUN 设备读取 -> 写入 VPN 连接
	go func() {
		defer wg.Done()
		common_utils.ProxyFromTunToVPN(tunDev, conn, errChan)
	}()

	// 等待错误或关闭
	err := <-errChan
	log.Printf("Proxying stopped for client %s: %v", clientID, err)
	conn.Close()

	// 等待两个代理 goroutine 完成
	wg.Wait()
	log.Printf("Finished handling client %s", clientID)
}

// // 从 VPN 连接读取数据包并写入 TUN 设备
// func proxyFromVPNToTun(conn *connectip.Conn, tunDev *common_utils.TUNDevice, errChan chan<- error) {
// 	b := make([]byte, 2048) // 读取数据包的缓冲区
// 	for {
// 		n, err := conn.ReadPacket(b)
// 		if err != nil {
// 			// 不要在设备有意关闭时报告错误
// 			var netErr *net.OpError
// 			var qErr *quic.ApplicationError
// 			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.As(err, &netErr) || errors.As(err, &qErr) {
// 				log.Println("Connection closed, stopping VPN->TUN proxy.")
// 				errChan <- nil
// 			} else {
// 				errChan <- fmt.Errorf("failed to read from connect-ip connection: %w", err)
// 			}
// 			return
// 		}
// 		if n == 0 {
// 			continue
// 		}

// 		// 写入 TUN 设备
// 		if _, err := tunDev.Write(b[:n]); err != nil {
// 			if errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
// 				log.Println("TUN device closed, stopping VPN->TUN proxy.")
// 				errChan <- nil
// 			} else {
// 				errChan <- fmt.Errorf("failed to write packet to TUN device %s: %w", tunDev.Name(), err)
// 			}
// 			return
// 		}
// 	}
// }

// // 从 TUN 设备读取数据包并写入 VPN 连接
// func proxyFromTunToVPN(tunDev *common_utils.TUNDevice, conn *connectip.Conn, errChan chan<- error) {
// 	b := make([]byte, 2048)
// 	for {
// 		n, err := tunDev.Read(b)
// 		if err != nil {
// 			// 不要在设备有意关闭时报告错误
// 			if errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) {
// 				log.Println("TUN device closed, stopping TUN->VPN proxy.")
// 				errChan <- nil // 发送干净关闭的信号
// 			} else {
// 				errChan <- fmt.Errorf("failed to read from TUN device %s: %w", tunDev.Name(), err)
// 			}
// 			return
// 		}
// 		if n == 0 {
// 			continue
// 		}

// 		// 向VPN连接写入数据包
// 		icmp, err := conn.WritePacket(b[:n])
// 		if err != nil {
// 			// 不要在连接有意关闭时报告错误
// 			var netErr *net.OpError
// 			var qErr *quic.ApplicationError
// 			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.As(err, &netErr) || errors.As(err, &qErr) {
// 				log.Println("Connection closed, stopping TUN->VPN proxy.")
// 				errChan <- nil
// 			} else {
// 				errChan <- fmt.Errorf("failed to write packet to connect-ip connection: %w", err)
// 			}
// 			return
// 		}

// 		// 如果 connect-ip 生成了 ICMP 响应，将其写回 TUN 设备
// 		if len(icmp) > 0 {
// 			log.Printf("Writing ICMP packet (%d bytes) from connect-ip back to TUN %s", len(icmp), tunDev.Name())
// 			if _, err := tunDev.Write(icmp); err != nil {
// 				log.Printf("Warning: failed to write ICMP packet to TUN %s: %v", tunDev.Name(), err)
// 				// 不要为此杀死连接，只记录日志
// 			}
// 		}
// 	}
// }
