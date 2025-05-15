package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	connectip "github.com/iselt/connect-ip-go"
	common "github.com/iselt/masque-vpn/common" // Import local module
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/yosida95/uritemplate/v3"
)

var serverConfig common.ServerConfig

func main() {
	if os.Getenv("PERF_PROFILE") != "" {
		f, _ := os.OpenFile("cpu.pprof", os.O_CREATE|os.O_RDWR, 0666)
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	// --- 配置加载 ---
	configFile := flag.String("c", "config.server.toml", "Config file path")
	flag.Parse()
	if _, err := toml.DecodeFile(*configFile, &serverConfig); err != nil {
		log.Fatalf("Error loading config file %s: %v", *configFile, err)
	}

	// --- 基础验证 ---
	if serverConfig.ListenAddr == "" || serverConfig.CertFile == "" || serverConfig.KeyFile == "" ||
		serverConfig.AssignCIDR == "" || serverConfig.ServerName == "" {
		log.Fatal("Missing required configuration values in config.server.toml")
	}

	// --- 创建 IP 分配器 ---
	networkInfo, err := common.NewNetworkInfo(serverConfig.AssignCIDR)
	if err != nil {
		log.Fatalf("Failed to create IP allocator: %v", err)
	}

	// 新增：创建全局 IP 地址池
	ipPool := common.NewIPPool(networkInfo.GetPrefix(), networkInfo.GetGateway().Addr())
	clientIPMap := make(map[string]netip.Addr)        // clientID -> IP
	ipConnMap := make(map[netip.Addr]*connectip.Conn) // IP -> conn
	var ipPoolMu sync.Mutex

	// --- 创建 TUN 设备 ---
	tunDev, err := common.CreateTunDevice(serverConfig.TunName, networkInfo.GetGateway(), serverConfig.MTU)
	if err != nil {
		log.Fatalf("Failed to create TUN device: %v", err)
	}
	defer tunDev.Close()

	// 启动TUN->VPN分发goroutine（修正位置）
	go func() {
		buf := make([]byte, 2048)
		for {
			n, err := tunDev.ReadPacket(buf, 0)
			if err != nil && err != os.ErrClosed {
				log.Printf("TUN read error: %v", err)
				continue
			}
			if n == 0 {
				continue
			}
			packet := make([]byte, n)
			copy(packet, buf[:n])

			// 提取目标IP
			dstIP, err := common.GetDestinationIP(packet, n)
			if err != nil {
				log.Printf("Failed to parse destination IP: %v", err)
				continue
			}

			ipPoolMu.Lock()
			conn, ok := ipConnMap[dstIP]
			ipPoolMu.Unlock()
			if ok {
				_, err := conn.WritePacket(packet)
				if err != nil {
					log.Printf("Failed to forward packet to client %s: %v", dstIP, err)
				}
			} else {
				// log.Printf("No client found for destination IP %s", dstIP)
			}
		}
	}()

	log.Printf("Starting VPN Server...")
	log.Printf("Listen Address: %s", serverConfig.ListenAddr)
	log.Printf("VPN Network: %s", networkInfo.GetPrefix())
	log.Printf("Gateway IP: %s", networkInfo.GetGateway())
	log.Printf("Advertised Routes: %v", serverConfig.AdvertiseRoutes)

	// --- 准备路由信息 ---
	var routesToAdvertise []connectip.IPRoute
	for _, routeStr := range serverConfig.AdvertiseRoutes {
		prefix, err := netip.ParsePrefix(routeStr)
		if err != nil {
			log.Fatalf("Invalid route in advertise_routes '%s': %v", routeStr, err)
		}
		routesToAdvertise = append(routesToAdvertise, connectip.IPRoute{
			StartIP:    prefix.Addr(),
			EndIP:      common.LastIP(prefix),
			IPProtocol: 0, // 0 表示任何协议
		})
	}

	// --- TLS 配置 ---
	var cert tls.Certificate
	if serverConfig.CertPEM != "" && serverConfig.KeyPEM != "" {
		cert, err = tls.X509KeyPair([]byte(serverConfig.CertPEM), []byte(serverConfig.KeyPEM))
		if err != nil {
			log.Fatalf("Failed to load TLS certificate/key from config PEM: %v", err)
		}
		log.Printf("Loaded TLS certificate/key from config PEM")
	} else {
		cert, err = tls.LoadX509KeyPair(serverConfig.CertFile, serverConfig.KeyFile)
		if err != nil {
			log.Fatalf("Failed to load TLS certificate/key: %v", err)
		}
		log.Printf("Loaded TLS certificate/key from file: %s, %s", serverConfig.CertFile, serverConfig.KeyFile)
	}

	// 加载CA证书
	caCertPool := x509.NewCertPool()
	var caCert []byte
	if serverConfig.CACertPEM != "" {
		caCert = []byte(serverConfig.CACertPEM)
		log.Printf("Loaded CA cert from config PEM")
	} else {
		if serverConfig.CACertFile == "" {
			log.Fatal("ca_file is required for mutual TLS authentication")
		}
		caCert, err = os.ReadFile(serverConfig.CACertFile)
		if err != nil {
			log.Fatalf("Failed to read CA file %s: %v", serverConfig.CACertFile, err)
		}
		log.Printf("Loaded CA cert from file: %s", serverConfig.CACertFile)
	}
	if !caCertPool.AppendCertsFromPEM(caCert) {
		log.Fatalf("Failed to append CA cert")
	}

	tlsConfig := http3.ConfigureTLSConfig(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
	})

	// --- QUIC 配置 ---
	quicConf := &quic.Config{
		EnableDatagrams: true,
		MaxIdleTimeout:  60 * time.Second,
		KeepAlivePeriod: 30 * time.Second,
	}

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

	log.Printf("Creating QUIC listener on %s...", serverConfig.ListenAddr)
	ln, err := quic.ListenEarly(udpConn, tlsConfig, quicConf)
	if err != nil {
		log.Fatalf("Failed to create QUIC listener: %v", err)
	}
	defer ln.Close()
	log.Printf("QUIC Listener started on %s", udpConn.LocalAddr())

	// --- CONNECT-IP 代理和 HTTP 处理程序 ---
	p := connectip.Proxy{}
	// 使用配置的服务器名称和端口作为模板
	serverHost, serverPortStr, _ := net.SplitHostPort(serverConfig.ListenAddr)
	if serverHost == "0.0.0.0" || serverHost == "[::]" || serverHost == "" {
		serverHost = serverConfig.ServerName // 如果监听的是通配符地址，使用配置的名称
	}
	serverPort, _ := strconv.Atoi(serverPortStr)
	template := uritemplate.MustNew(fmt.Sprintf("https://%s:%d/vpn", serverHost, serverPort))

	mux := http.NewServeMux()
	mux.HandleFunc("/vpn", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Incoming VPN request from %s", r.RemoteAddr)
		// 新增：提取 client_id（证书 CommonName）
		var clientID string
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			clientID = r.TLS.PeerCertificates[0].Subject.CommonName
		} else {
			http.Error(w, "未检测到客户端证书", http.StatusUnauthorized)
			return
		}
		// 新增：校验 client_id 是否在数据库中
		clientIDExists := func(id string) bool {
			db, err := sql.Open("sqlite3", serverConfig.APIServer.DatabasePath) // 使用配置的数据库路径
			if err != nil {
				log.Printf("Error opening database: %v", err) // Log the error
				return false
			}
			defer db.Close()
			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM clients WHERE client_id = ?", id).Scan(&count)
			if err != nil {
				log.Printf("Error querying database for client_id %s: %v", id, err) // Log the error
				return false
			}
			return count > 0
		}
		if !clientIDExists(clientID) {
			log.Printf("拒绝未知 client_id 连接: %s", clientID)
			http.Error(w, "客户端未授权或已被删除", http.StatusUnauthorized)
			return
		}

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

		log.Printf("CONNECT-IP session established for %s", clientID)

		// 新增：为客户端分配唯一 IP
		ipPoolMu.Lock()
		assignedPrefix, allocErr := ipPool.Allocate(clientID)
		if allocErr != nil {
			ipPoolMu.Unlock()
			log.Printf("No available IP for client %s: %v", clientID, allocErr)
			conn.Close()
			return
		}
		clientIPMap[clientID] = assignedPrefix.Addr()
		ipConnMap[assignedPrefix.Addr()] = conn
		ipPoolMu.Unlock()
		log.Printf("Allocated IP %s to client %s", assignedPrefix, clientID)

		// 处理客户端连接，传递分配的 IP 和数据库路径
		go handleClientConnection(conn, clientID, tunDev, assignedPrefix, routesToAdvertise, ipPool, &ipPoolMu, clientIPMap, ipConnMap, serverConfig.APIServer.DatabasePath)
	})

	// 新增：API服务goroutine
	go func() {
		// 传递 serverConfig 给 API Server，并传递监听地址
		StartAPIServer(&ipPoolMu, clientIPMap, ipConnMap, serverConfig)
	}()

	// --- HTTP/3 Server ---
	h3Server := http3.Server{
		Handler:         mux,
		EnableDatagrams: true,
		QUICConfig:      quicConf, // 使用与客户端相同的QUIC配置
	}

	// --- 优雅关闭处理 ---
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

	// 等待关闭信号
	<-ctx.Done()
	log.Println("Shutdown signal received...")

	// 初始化优雅关闭
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
	defer cancel()

	if err := h3Server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP/3 server shutdown ungracefully: %v", err)
	} else {
		log.Println("HTTP/3 server shutdown gracefully.")
	}

	// 等待服务器goroutine结束
	wg.Wait()
	log.Println("VPN Server exited.")
}

// handleClientConnection 处理客户端VPN连接
func handleClientConnection(conn *connectip.Conn, clientID string,
	tunDev *common.TUNDevice, assignedPrefix netip.Prefix, routes []connectip.IPRoute,
	ipPool *common.IPPool, ipPoolMu *sync.Mutex, clientIPMap map[string]netip.Addr, ipConnMap map[netip.Addr]*connectip.Conn, dbPath string) { // 新增 dbPath 参数
	defer conn.Close()

	log.Printf("Handling connection for client %s", clientID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// --- 为客户端分配唯一 IP 前缀 ---
	if err := conn.AssignAddresses(ctx, []netip.Prefix{assignedPrefix}); err != nil {
		log.Printf("Error assigning address %s to client %s: %v", assignedPrefix, clientID, err)
		// 释放 IP
		ipPoolMu.Lock()
		ipPool.Release(assignedPrefix.Addr())
		delete(clientIPMap, clientID)
		delete(ipConnMap, assignedPrefix.Addr())
		ipPoolMu.Unlock()
		return
	}
	log.Printf("Assigned IP %s to client %s", assignedPrefix, clientID)

	// --- 向客户端广播路由 ---
	if err := conn.AdvertiseRoute(ctx, routes); err != nil {
		log.Printf("Error advertising routes to client %s: %v", clientID, err)
		ipPoolMu.Lock()
		ipPool.Release(assignedPrefix.Addr())
		delete(clientIPMap, clientID)
		delete(ipConnMap, assignedPrefix.Addr())
		ipPoolMu.Unlock()
		return
	}
	log.Printf("Advertised %d routes to client %s", len(routes), clientID)

	// --- 用户组与访问控制策略 ---
	// 修改：调用 api_server.go 中的 getGroupsAndPoliciesForClient，并传递 dbPath
	groupIDs, policies := getGroupsAndPoliciesForClient(dbPath, clientID)
	conn.SetAccessControl(clientID, groupIDs, policies)

	// --- 只保留VPN->TUN方向 ---
	errChan := make(chan error, 1)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		common.ProxyFromVPNToTun(tunDev, conn, errChan)
	}()

	err := <-errChan
	log.Printf("Proxying stopped for client %s: %v", clientID, err)
	conn.Close()

	wg.Wait()
	log.Printf("Finished handling client %s", clientID)

	// 连接结束时释放 IP
	ipPoolMu.Lock()
	// 修复：只有当 clientID 还在 clientIPMap 时才释放，避免被 API 删除后重复释放
	if ip, ok := clientIPMap[clientID]; ok && ip == assignedPrefix.Addr() {
		ipPool.Release(assignedPrefix.Addr())
		delete(clientIPMap, clientID)
		delete(ipConnMap, assignedPrefix.Addr())
	}
	ipPoolMu.Unlock()
}

// getGroupsAndPoliciesForClient 也需要 dbPath 参数
func getGroupsAndPoliciesForClient(dbPath string, clientID string) ([]string, []connectip.AccessPolicy) {
	groupIDs := []string{}
	policies := []connectip.AccessPolicy{}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Printf("[ACL] 打开数据库失败: %v", err)
		return groupIDs, policies
	}
	defer db.Close()
	// 查询groupIDs
	rows, err := db.Query("SELECT group_id FROM group_members WHERE client_id = ?", clientID)
	if err == nil {
		for rows.Next() {
			var gid string
			if err := rows.Scan(&gid); err == nil {
				groupIDs = append(groupIDs, gid)
			}
		}
		rows.Close()
	}
	if len(groupIDs) == 0 {
		return groupIDs, policies
	}
	// 查询所有相关策略
	query, args := "SELECT action, ip_prefix, priority FROM access_policies WHERE group_id IN ("+strings.Repeat("?,", len(groupIDs)-1)+"?) ORDER BY priority ASC", make([]interface{}, len(groupIDs))
	for i, gid := range groupIDs {
		args[i] = gid
	}
	rows, err = db.Query(query, args...)
	if err == nil {
		for rows.Next() {
			var action, ipPrefix string
			var priority int
			if err := rows.Scan(&action, &ipPrefix, &priority); err == nil {
				if prefix, err := netip.ParsePrefix(ipPrefix); err == nil {
					policies = append(policies, connectip.AccessPolicy{
						Action:   action,
						IPPrefix: prefix,
						Priority: priority,
					})
				}
			}
		}
		rows.Close()
	}
	return groupIDs, policies
}
