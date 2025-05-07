package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/hex"
	"encoding/pem"
	"log"
	"math/big"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	connectip "github.com/iselt/connect-ip-go"
	common "github.com/iselt/masque-vpn/common"
	_ "github.com/mattn/go-sqlite3"
)

// 常量
const dbFile = "masque_admin.db"

// 类型定义

// 客户端统计信息
type ClientStats struct {
	IP       string `json:"ip"`
	ClientID string `json:"client_id"`
	Online   bool   `json:"online"`
	BytesIn  uint64 `json:"bytes_in"`
	BytesOut uint64 `json:"bytes_out"`
	LastSeen int64  `json:"last_seen"`
}

// 服务器配置
type ServerConfigDB struct {
	ServerAddr string `json:"server_addr"`
	ServerName string `json:"server_name"`
	MTU        int    `json:"mtu"`
}

// 全局变量
var (
	globalClientIPMap = make(map[string]netip.Addr)
	globalIPConnMap   = make(map[netip.Addr]*connectip.Conn)
	// 会话存储
	sessionStore = make(map[string]string)
	sessionMu    sync.Mutex
)

// 数据库相关函数
func initDB() {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatalf("数据库打开失败: %v", err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS admin (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE,
		password TEXT
	)`)
	if err != nil {
		log.Fatalf("创建admin表失败: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS clients (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		client_id TEXT UNIQUE,
		cert_pem TEXT,
		key_pem TEXT,
		config TEXT,
		created_at DATETIME
	)`)
	if err != nil {
		log.Fatalf("创建clients表失败: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS server_config (
		id INTEGER PRIMARY KEY,
		server_addr TEXT,
		server_name TEXT,
		mtu INTEGER
	)`)
	if err != nil {
		log.Fatalf("创建server_config表失败: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS groups (
		group_id TEXT PRIMARY KEY,
		group_name TEXT UNIQUE
	)`)
	if err != nil {
		log.Fatalf("创建groups表失败: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS group_members (
		group_id TEXT,
		client_id TEXT,
		PRIMARY KEY (group_id, client_id)
	)`)
	if err != nil {
		log.Fatalf("创建group_members表失败: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS access_policies (
		policy_id TEXT PRIMARY KEY,
		group_id TEXT,
		action TEXT,
		ip_prefix TEXT,
		priority INTEGER
	)`)
	if err != nil {
		log.Fatalf("创建access_policies表失败: %v", err)
	}
	var count int
	db.QueryRow("SELECT COUNT(*) FROM admin WHERE username = 'admin'").Scan(&count)
	if count == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		_, err = db.Exec("INSERT INTO admin(username, password) VALUES (?, ?)", "admin", string(hash))
		if err != nil {
			log.Fatalf("插入默认管理员失败: %v", err)
		}
		log.Println("已初始化默认管理员账号：admin/admin")
	}
}

func getServerConfigFromDB() (ServerConfigDB, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return ServerConfigDB{}, err
	}
	defer db.Close()
	row := db.QueryRow("SELECT server_addr, server_name, mtu FROM server_config WHERE id=1")
	var cfg ServerConfigDB
	var mtu sql.NullInt64
	if err := row.Scan(&cfg.ServerAddr, &cfg.ServerName, &mtu); err != nil {
		return ServerConfigDB{}, err
	}
	if mtu.Valid {
		cfg.MTU = int(mtu.Int64)
	} else {
		cfg.MTU = 1413
	}
	return cfg, nil
}

func saveServerConfigToDB(cfg ServerConfigDB) error {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`INSERT INTO server_config (id, server_addr, server_name, mtu) VALUES (1,?,?,?)
		ON CONFLICT(id) DO UPDATE SET server_addr=excluded.server_addr, server_name=excluded.server_name, mtu=excluded.mtu`,
		cfg.ServerAddr, cfg.ServerName, cfg.MTU)
	return err
}

// 会话/认证相关函数
func checkAdminLogin(username, password string) bool {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return false
	}
	defer db.Close()
	var hash string
	err = db.QueryRow("SELECT password FROM admin WHERE username = ?", username).Scan(&hash)
	if err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func ginSetSession(c *gin.Context, username string) {
	sid := generateSessionID()
	sessionMu.Lock()
	sessionStore[sid] = username
	sessionMu.Unlock()
	c.SetCookie("masque_admin_sid", sid, 3600*24, "/", "", false, true)
}

func ginCheckSession(c *gin.Context) bool {
	sid, err := c.Cookie("masque_admin_sid")
	if err != nil {
		return false
	}
	sessionMu.Lock()
	_, ok := sessionStore[sid]
	sessionMu.Unlock()
	return ok
}

func ginRequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ginCheckSession(c) {
			c.AbortWithStatusJSON(401, gin.H{"error": "未登录或会话已过期"})
			return
		}
		c.Next()
	}
}

// Gin API 处理函数
// 登录
func ginHandleLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}
	if checkAdminLogin(req.Username, req.Password) {
		ginSetSession(c, req.Username)
		c.JSON(200, gin.H{"success": true})
	} else {
		c.JSON(401, gin.H{"error": "用户名或密码错误"})
	}
}

// 客户端相关
func ginHandleListClients(clientIPMap map[string]netip.Addr) gin.HandlerFunc {
	return func(c *gin.Context) {
		db, err := sql.Open("sqlite3", dbFile)
		if err != nil {
			c.JSON(500, gin.H{"error": "数据库错误"})
			return
		}
		defer db.Close()
		rows, err := db.Query("SELECT client_id, created_at FROM clients ORDER BY created_at DESC")
		if err != nil {
			c.JSON(500, gin.H{"error": "查询失败"})
			return
		}
		defer rows.Close()
		var clients []map[string]interface{}
		for rows.Next() {
			var clientID, createdAt string
			rows.Scan(&clientID, &createdAt)
			_, online := clientIPMap[clientID]
			clients = append(clients, map[string]interface{}{
				"client_id":  clientID,
				"created_at": createdAt,
				"online":     online,
			})
		}
		c.JSON(200, clients)
	}
}

func ginHandleGenClientV2(serverConfig interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := serverConfig.(common.ServerConfig)
		var caCertPEM, caKeyPEM []byte
		var err error
		if cfg.CACertPEM != "" && cfg.CAKeyPEM != "" {
			caCertPEM = []byte(cfg.CACertPEM)
			caKeyPEM = []byte(cfg.CAKeyPEM)
		} else {
			caCertPEM, err = os.ReadFile(cfg.CACertFile)
			if err != nil {
				c.JSON(500, gin.H{"error": "CA证书不存在，请先生成CA"})
				return
			}
			caKeyPEM, err = os.ReadFile(cfg.CAKeyFile)
			if err != nil {
				c.JSON(500, gin.H{"error": "CA私钥不存在，请先生成CA"})
				return
			}
		}
		block, _ := pem.Decode(caKeyPEM)
		if block == nil {
			c.JSON(500, gin.H{"error": "CA私钥格式错误"})
			return
		}
		var caKey *rsa.PrivateKey
		if block.Type == "RSA PRIVATE KEY" {
			caKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				c.JSON(500, gin.H{"error": "解析CA私钥失败"})
				return
			}
		} else if block.Type == "PRIVATE KEY" {
			keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				c.JSON(500, gin.H{"error": "解析PKCS#8 CA私钥失败"})
				return
			}
			var ok bool
			caKey, ok = keyAny.(*rsa.PrivateKey)
			if !ok {
				c.JSON(500, gin.H{"error": "CA私钥不是RSA类型"})
				return
			}
		} else {
			c.JSON(500, gin.H{"error": "CA私钥格式错误(未知类型)"})
			return
		}
		caBlock, _ := pem.Decode(caCertPEM)
		if caBlock == nil || caBlock.Type != "CERTIFICATE" {
			c.JSON(500, gin.H{"error": "CA证书格式错误"})
			return
		}
		caCert, err := x509.ParseCertificate(caBlock.Bytes)
		if err != nil {
			c.JSON(500, gin.H{"error": "解析CA证书失败"})
			return
		}
		clientPriv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			c.JSON(500, gin.H{"error": "生成客户端私钥失败"})
			return
		}
		clientID := uuid.NewString()
		clientTemplate := x509.Certificate{
			SerialNumber: big.NewInt(time.Now().UnixNano()),
			Subject: pkix.Name{
				Organization: []string{"MasqueVPN Client"},
				CommonName:   clientID,
			},
			NotBefore:   time.Now(),
			NotAfter:    time.Now().Add(3 * 365 * 24 * time.Hour),
			KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
		clientDER, err := x509.CreateCertificate(rand.Reader, &clientTemplate, caCert, &clientPriv.PublicKey, caKey)
		if err != nil {
			c.JSON(500, gin.H{"error": "生成客户端证书失败"})
			return
		}
		clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER})
		clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientPriv)})

		tmplBytes, err := os.ReadFile("config.client.toml.example")
		if err != nil {
			c.JSON(500, gin.H{"error": "找不到客户端配置模板"})
			return
		}
		tmpl := string(tmplBytes)
		repl := map[string]string{
			"server_addr":          c.Query("server_addr"),
			"server_name":          c.Query("server_name"),
			"mtu":                  c.Query("mtu"),
			"ca_pem":               string(caCertPEM),
			"cert_pem":             string(clientCertPEM),
			"key_pem":              string(clientKeyPEM),
			"key_log_file":         c.Query("key_log_file"),
			"log_level":            c.Query("log_level"),
			"insecure_skip_verify": c.Query("insecure_skip_verify"),
			"tun_name":             c.Query("tun_name"),
		}
		if repl["server_addr"] == "" {
			repl["server_addr"] = "<请填写VPN服务器地址:端口>"
		}
		if repl["server_name"] == "" {
			repl["server_name"] = "<请填写服务器名称>"
		}
		if repl["mtu"] == "" {
			repl["mtu"] = "1413"
		}
		if repl["log_level"] == "" {
			repl["log_level"] = "info"
		}
		if repl["insecure_skip_verify"] == "" {
			repl["insecure_skip_verify"] = "false"
		}
		for k, v := range repl {
			tmpl = strings.ReplaceAll(tmpl, "{{"+k+"}}", v)
		}
		config := tmpl

		db, err := sql.Open("sqlite3", dbFile)
		if err != nil {
			c.JSON(500, gin.H{"error": "数据库错误"})
			return
		}
		defer db.Close()
		_, err = db.Exec("INSERT INTO clients(client_id, cert_pem, key_pem, config, created_at) VALUES (?, ?, ?, ?, datetime('now'))",
			clientID, string(clientCertPEM), string(clientKeyPEM), config)
		if err != nil {
			c.JSON(500, gin.H{"error": "写入数据库失败"})
			return
		}
		c.JSON(200, gin.H{"client_id": clientID})
	}
}

func ginHandleDownloadClient(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "缺少id参数"})
		return
	}
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		c.JSON(500, gin.H{"error": "数据库错误"})
		return
	}
	defer db.Close()
	var config string
	err = db.QueryRow("SELECT config FROM clients WHERE client_id = ?", id).Scan(&config)
	if err != nil {
		c.JSON(404, gin.H{"error": "未找到该客户端"})
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=config.client.toml")
	c.String(200, config)
}

func ginHandleDeleteClient(ipPoolMu *sync.Mutex, clientIPMap map[string]netip.Addr, ipConnMap map[netip.Addr]*connectip.Conn) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Query("id")
		if id == "" {
			c.JSON(400, gin.H{"error": "缺少id参数"})
			return
		}
		if ipPoolMu != nil && clientIPMap != nil && ipConnMap != nil {
			ipPoolMu.Lock()
			if ip, ok := clientIPMap[id]; ok {
				if conn, ok2 := ipConnMap[ip]; ok2 {
					log.Printf("主动断开客户端 %s (IP: %s) 的连接", id, ip)
					conn.Close()
					delete(ipConnMap, ip)
				}
				delete(clientIPMap, id)
			}
			ipPoolMu.Unlock()
		}
		db, err := sql.Open("sqlite3", dbFile)
		if err != nil {
			c.JSON(500, gin.H{"error": "数据库错误"})
			return
		}
		defer db.Close()
		_, err = db.Exec("DELETE FROM clients WHERE client_id = ?", id)
		if err != nil {
			c.JSON(500, gin.H{"error": "删除失败"})
			return
		}
		c.String(200, "ok")
	}
}

// 服务器配置相关
func ginHandleGetServerConfig(c *gin.Context) {
	cfg, err := getServerConfigFromDB()
	if err != nil {
		cfg = ServerConfigDB{ServerAddr: "", ServerName: "", MTU: 1413}
	}
	c.JSON(200, cfg)
}

func ginHandleSetServerConfig(c *gin.Context) {
	var req ServerConfigDB
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}
	if req.MTU < 576 || req.MTU > 9000 {
		c.JSON(400, gin.H{"error": "MTU不合法"})
		return
	}
	if err := saveServerConfigToDB(req); err != nil {
		c.JSON(500, gin.H{"error": "保存失败"})
		return
	}
	c.String(200, "ok")
}

// 分组相关
func ginHandleListGroups(c *gin.Context) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		c.JSON(500, gin.H{"error": "数据库错误"})
		return
	}
	defer db.Close()
	rows, err := db.Query("SELECT group_id, group_name FROM groups ORDER BY group_name")
	if err != nil {
		c.JSON(500, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()
	var groups []map[string]string
	for rows.Next() {
		var gid, gname string
		rows.Scan(&gid, &gname)
		groups = append(groups, map[string]string{"group_id": gid, "group_name": gname})
	}
	c.JSON(200, groups)
}

func ginHandleAddGroup(c *gin.Context) {
	var req struct {
		GroupName string `json:"group_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.GroupName == "" {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		c.JSON(500, gin.H{"error": "数据库错误"})
		return
	}
	defer db.Close()
	gid := uuid.NewString()
	_, err = db.Exec("INSERT INTO groups(group_id, group_name) VALUES (?, ?)", gid, req.GroupName)
	if err != nil {
		c.JSON(400, gin.H{"error": "添加失败，组名可能重复"})
		return
	}
	c.String(200, "ok")
}

func ginHandleDeleteGroup(c *gin.Context) {
	gid := c.Query("id")
	if gid == "" {
		c.JSON(400, gin.H{"error": "缺少id参数"})
		return
	}
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		c.JSON(500, gin.H{"error": "数据库错误"})
		return
	}
	defer db.Close()
	_, _ = db.Exec("DELETE FROM group_members WHERE group_id = ?", gid)
	_, err = db.Exec("DELETE FROM groups WHERE group_id = ?", gid)
	if err != nil {
		c.JSON(500, gin.H{"error": "删除失败"})
		return
	}
	c.String(200, "ok")
}

func ginHandleUpdateGroup(c *gin.Context) {
	var req struct{ GroupID, GroupName string }
	if err := c.ShouldBindJSON(&req); err != nil || req.GroupID == "" || req.GroupName == "" {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		c.JSON(500, gin.H{"error": "数据库错误"})
		return
	}
	defer db.Close()
	_, err = db.Exec("UPDATE groups SET group_name = ? WHERE group_id = ?", req.GroupName, req.GroupID)
	if err != nil {
		c.JSON(500, gin.H{"error": "更新失败"})
		return
	}
	c.String(200, "ok")
}

func ginHandleListGroupMembers(c *gin.Context) {
	gid := c.Query("group_id")
	if gid == "" {
		c.JSON(400, gin.H{"error": "缺少group_id参数"})
		return
	}
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		c.JSON(500, gin.H{"error": "数据库错误"})
		return
	}
	defer db.Close()
	rows, err := db.Query("SELECT client_id FROM group_members WHERE group_id = ?", gid)
	if err != nil {
		c.JSON(500, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()
	var members []string
	for rows.Next() {
		var cid string
		rows.Scan(&cid)
		members = append(members, cid)
	}
	c.JSON(200, members)
}

func ginHandleAddGroupMember(c *gin.Context) {
	var req struct{ GroupID, ClientID string }
	if err := c.ShouldBindJSON(&req); err != nil || req.GroupID == "" || req.ClientID == "" {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		c.JSON(500, gin.H{"error": "数据库错误"})
		return
	}
	defer db.Close()
	_, err = db.Exec("INSERT OR IGNORE INTO group_members(group_id, client_id) VALUES (?, ?)", req.GroupID, req.ClientID)
	if err != nil {
		c.JSON(500, gin.H{"error": "添加失败"})
		return
	}
	c.String(200, "ok")
}

func ginHandleRemoveGroupMember(c *gin.Context) {
	var req struct{ GroupID, ClientID string }
	if err := c.ShouldBindJSON(&req); err != nil || req.GroupID == "" || req.ClientID == "" {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		c.JSON(500, gin.H{"error": "数据库错误"})
		return
	}
	defer db.Close()
	_, err = db.Exec("DELETE FROM group_members WHERE group_id = ? AND client_id = ?", req.GroupID, req.ClientID)
	if err != nil {
		c.JSON(500, gin.H{"error": "移除失败"})
		return
	}
	c.String(200, "ok")
}

// 策略相关
func ginHandleListPolicies(c *gin.Context) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		c.JSON(500, gin.H{"error": "数据库错误"})
		return
	}
	defer db.Close()
	rows, err := db.Query("SELECT policy_id, group_id, action, ip_prefix, priority FROM access_policies ORDER BY priority ASC")
	if err != nil {
		c.JSON(500, gin.H{"error": "查询失败"})
		return
	}
	defer rows.Close()
	var policies []map[string]interface{}
	for rows.Next() {
		var pid, gid, action, ipPrefix string
		var priority int
		rows.Scan(&pid, &gid, &action, &ipPrefix, &priority)
		policies = append(policies, map[string]interface{}{
			"policy_id": pid, "group_id": gid, "action": action, "ip_prefix": ipPrefix, "priority": priority,
		})
	}
	c.JSON(200, policies)
}

func ginHandleAddPolicy(c *gin.Context) {
	var req struct {
		GroupID  string `json:"group_id"`
		Action   string `json:"action"`
		IPPrefix string `json:"ip_prefix"`
		Priority int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.GroupID == "" || req.Action == "" || req.IPPrefix == "" {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}
	if req.Action != "allow" && req.Action != "deny" {
		c.JSON(400, gin.H{"error": "action必须为allow或deny"})
		return
	}
	if _, err := netip.ParsePrefix(req.IPPrefix); err != nil {
		c.JSON(400, gin.H{"error": "ip_prefix格式错误"})
		return
	}
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		c.JSON(500, gin.H{"error": "数据库错误"})
		return
	}
	defer db.Close()
	pid := uuid.NewString()
	_, err = db.Exec("INSERT INTO access_policies(policy_id, group_id, action, ip_prefix, priority) VALUES (?, ?, ?, ?, ?)", pid, req.GroupID, req.Action, req.IPPrefix, req.Priority)
	if err != nil {
		c.JSON(500, gin.H{"error": "添加失败"})
		return
	}
	c.String(200, "ok")
	go refreshAccessControlForGroup(req.GroupID, globalClientIPMap, globalIPConnMap)
}

func ginHandleDeletePolicy(c *gin.Context) {
	pid := c.Query("id")
	if pid == "" {
		c.JSON(400, gin.H{"error": "缺少id参数"})
		return
	}
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		c.JSON(500, gin.H{"error": "数据库错误"})
		return
	}
	defer db.Close()
	var groupID string
	db.QueryRow("SELECT group_id FROM access_policies WHERE policy_id = ?", pid).Scan(&groupID)
	_, err = db.Exec("DELETE FROM access_policies WHERE policy_id = ?", pid)
	if err != nil {
		c.JSON(500, gin.H{"error": "删除失败"})
		return
	}
	c.String(200, "ok")
	if groupID != "" {
		go refreshAccessControlForGroup(groupID, globalClientIPMap, globalIPConnMap)
	}
}

func ginHandleUpdatePolicy(c *gin.Context) {
	var req struct {
		PolicyID string `json:"policy_id"`
		GroupID  string `json:"group_id"`
		Action   string `json:"action"`
		IPPrefix string `json:"ip_prefix"`
		Priority int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.PolicyID == "" || req.GroupID == "" || req.Action == "" || req.IPPrefix == "" {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}
	if req.Action != "allow" && req.Action != "deny" {
		c.JSON(400, gin.H{"error": "action必须为allow或deny"})
		return
	}
	if _, err := netip.ParsePrefix(req.IPPrefix); err != nil {
		c.JSON(400, gin.H{"error": "ip_prefix格式错误"})
		return
	}
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		c.JSON(500, gin.H{"error": "数据库错误"})
		return
	}
	defer db.Close()
	_, err = db.Exec("UPDATE access_policies SET group_id=?, action=?, ip_prefix=?, priority=? WHERE policy_id=?", req.GroupID, req.Action, req.IPPrefix, req.Priority, req.PolicyID)
	if err != nil {
		c.JSON(500, gin.H{"error": "更新失败"})
		return
	}
	c.String(200, "ok")
	go refreshAccessControlForGroup(req.GroupID, globalClientIPMap, globalIPConnMap)
}

// 访问控制/辅助函数
func refreshAccessControlForGroup(groupID string, clientIPMap map[string]netip.Addr, ipConnMap map[netip.Addr]*connectip.Conn) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Printf("[ACL] 刷新策略时打开数据库失败: %v", err)
		return
	}
	defer db.Close()
	// 1. 查找所有属于 groupID 的 client_id
	rows, err := db.Query("SELECT client_id FROM group_members WHERE group_id = ?", groupID)
	if err != nil {
		log.Printf("[ACL] 查询group成员失败: %v", err)
		return
	}
	defer rows.Close()
	var clientIDs []string
	for rows.Next() {
		var cid string
		if err := rows.Scan(&cid); err == nil {
			clientIDs = append(clientIDs, cid)
		}
	}
	// 2. 遍历 clientIPMap，找到在线的 client_id
	for _, clientID := range clientIDs {
		if ip, ok := clientIPMap[clientID]; ok {
			if conn, ok2 := ipConnMap[ip]; ok2 {
				// 3. 重新查 groupIDs 和 policies
				groupIDs, policies := getGroupsAndPoliciesForClient(clientID)
				conn.SetAccessControl(clientID, groupIDs, policies)
				log.Printf("[ACL] 已刷新客户端 %s 的访问控制策略", clientID)
			}
		}
	}
}

// 主启动函数
func StartAPIServer(ipPoolMu *sync.Mutex, clientIPMap map[string]netip.Addr, ipConnMap map[netip.Addr]*connectip.Conn, serverConfig interface{}) {
	globalClientIPMap = clientIPMap
	globalIPConnMap = ipConnMap

	initDB()
	// 初始化 Gin
	r := gin.Default()

	// 静态文件服务
	r.Static("/", "./web")

	// API 路由分组
	api := r.Group("/api")
	{
		api.POST("/login", ginHandleLogin)

		// 需要认证的接口
		auth := api.Group("").Use(ginRequireAuth())
		{
			auth.GET("/clients", ginHandleListClients(clientIPMap))
			auth.POST("/gen_client", ginHandleGenClientV2(serverConfig))
			auth.GET("/download_client", ginHandleDownloadClient)
			auth.POST("/delete_client", ginHandleDeleteClient(ipPoolMu, clientIPMap, ipConnMap))

			auth.GET("/server_config", ginHandleGetServerConfig)
			auth.POST("/server_config", ginHandleSetServerConfig)

			auth.GET("/groups", ginHandleListGroups)
			auth.POST("/groups", ginHandleAddGroup)
			auth.POST("/groups/delete", ginHandleDeleteGroup)
			auth.POST("/groups/update", ginHandleUpdateGroup)

			auth.GET("/groups/members", ginHandleListGroupMembers)
			auth.POST("/groups/members", ginHandleAddGroupMember)
			auth.POST("/groups/members/remove", ginHandleRemoveGroupMember)

			auth.GET("/policies", ginHandleListPolicies)
			auth.POST("/policies", ginHandleAddPolicy)
			auth.POST("/policies/delete", ginHandleDeletePolicy)
			auth.POST("/policies/update", ginHandleUpdatePolicy)
		}
	}

	// 读取监听地址
	listenAddr := ":8080" // 默认
	if cfg, ok := serverConfig.(common.ServerConfig); ok && cfg.APIServerListenAddr != "" {
		listenAddr = cfg.APIServerListenAddr
	}
	log.Printf("API server (Gin) listening on %s ...", listenAddr)
	r.Run(listenAddr)
}
