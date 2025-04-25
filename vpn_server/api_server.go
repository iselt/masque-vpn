package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"log"
	"math/big"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	_ "github.com/mattn/go-sqlite3"
	connectip "github.com/quic-go/connect-ip-go" // 新增导入
)

type ClientStats struct {
	IP       string `json:"ip"`
	ClientID string `json:"client_id"`
	Online   bool   `json:"online"`
	BytesIn  uint64 `json:"bytes_in"`
	BytesOut uint64 `json:"bytes_out"`
	LastSeen int64  `json:"last_seen"`
}

const dbFile = "masque_admin.db"

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
	// 新增：客户端表
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
	// 新增：服务器配置表
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS server_config (
		id INTEGER PRIMARY KEY,
		server_addr TEXT,
		server_name TEXT,
		mtu INTEGER
	)`)
	if err != nil {
		log.Fatalf("创建server_config表失败: %v", err)
	}
	// 检查是否有admin账号
	var count int
	db.QueryRow("SELECT COUNT(*) FROM admin WHERE username = 'admin'").Scan(&count)
	if count == 0 {
		// 默认密码admin，使用bcrypt加密
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		_, err = db.Exec("INSERT INTO admin(username, password) VALUES (?, ?)", "admin", string(hash))
		if err != nil {
			log.Fatalf("插入默认管理员失败: %v", err)
		}
		log.Println("已初始化默认管理员账号：admin/admin")
	}
}

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

var (
	sessionStore = make(map[string]string) // sessionID -> username
	sessionMu    sync.Mutex
)

func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func setSession(w http.ResponseWriter, username string) {
	sid := generateSessionID()
	sessionMu.Lock()
	sessionStore[sid] = username
	sessionMu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     "masque_admin_sid",
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func checkSession(r *http.Request) bool {
	cookie, err := r.Cookie("masque_admin_sid")
	if err != nil {
		return false
	}
	sessionMu.Lock()
	_, ok := sessionStore[cookie.Value]
	sessionMu.Unlock()
	return ok
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkSession(r) {
			w.WriteHeader(401)
			w.Write([]byte("未登录或会话已过期"))
			return
		}
		next(w, r)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持POST", 405)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "参数错误", 400)
		return
	}
	if checkAdminLogin(req.Username, req.Password) {
		setSession(w, req.Username)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
	} else {
		http.Error(w, "用户名或密码错误", 401)
	}
}

func handleCAStatus(w http.ResponseWriter, r *http.Request) {
	_, err1 := os.Stat("ca.crt")
	_, err2 := os.Stat("ca.key")
	exists := err1 == nil && err2 == nil
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"exists": exists})
}

// 获取所有客户端信息（含在线状态）
func handleListClients(clientIPMap map[string]netip.Addr) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db, err := sql.Open("sqlite3", dbFile)
		if err != nil {
			http.Error(w, "数据库错误", 500)
			return
		}
		defer db.Close()
		rows, err := db.Query("SELECT client_id, created_at FROM clients ORDER BY created_at DESC")
		if err != nil {
			http.Error(w, "查询失败", 500)
			return
		}
		defer rows.Close()
		var clients []map[string]interface{}
		for rows.Next() {
			var clientID string
			var createdAt string
			rows.Scan(&clientID, &createdAt)
			// 修正：判断在线状态 - 检查 clientID 是否在 clientIPMap 的键中
			_, online := clientIPMap[clientID]
			clients = append(clients, map[string]interface{}{
				"client_id":  clientID,
				"created_at": createdAt,
				"online":     online,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(clients)
	}
}

// 生成客户端证书并写入数据库，返回client_id
func handleGenClientV2(w http.ResponseWriter, r *http.Request) {
	// ...证书生成逻辑同前...
	caCertPEM, err := os.ReadFile("ca.crt")
	if err != nil {
		http.Error(w, "CA证书不存在，请先生成CA", 500)
		return
	}
	caKeyPEM, err := os.ReadFile("ca.key")
	if err != nil {
		http.Error(w, "CA私钥不存在，请先生成CA", 500)
		return
	}
	block, _ := pem.Decode(caKeyPEM)
	if block == nil {
		http.Error(w, "CA私钥格式错误", 500)
		return
	}
	var caKey *rsa.PrivateKey
	if block.Type == "RSA PRIVATE KEY" {
		caKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			http.Error(w, "解析CA私钥失败", 500)
			return
		}
	} else if block.Type == "PRIVATE KEY" {
		keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			http.Error(w, "解析PKCS#8 CA私钥失败", 500)
			return
		}
		var ok bool
		caKey, ok = keyAny.(*rsa.PrivateKey)
		if !ok {
			http.Error(w, "CA私钥不是RSA类型", 500)
			return
		}
	} else {
		http.Error(w, "CA私钥格式错误(未知类型)", 500)
		return
	}
	caBlock, _ := pem.Decode(caCertPEM)
	if caBlock == nil || caBlock.Type != "CERTIFICATE" {
		http.Error(w, "CA证书格式错误", 500)
		return
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		http.Error(w, "解析CA证书失败", 500)
		return
	}
	clientPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		http.Error(w, "生成客户端私钥失败", 500)
		return
	}
	// 生成8位字母数字clientID
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, 8)
	randBytes := make([]byte, 8)
	rand.Read(randBytes)
	for i := 0; i < 8; i++ {
		b[i] = letters[int(randBytes[i])%len(letters)]
	}
	clientID := "client-" + string(b)
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
		http.Error(w, "生成客户端证书失败", 500)
		return
	}
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER})
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientPriv)})

	// 读取模板并替换
	tmplBytes, err := os.ReadFile("config.client.toml.example")
	if err != nil {
		http.Error(w, "找不到客户端配置模板", 500)
		return
	}
	tmpl := string(tmplBytes)
	q := r.URL.Query()
	// 构造替换map
	repl := map[string]string{
		"server_addr":          q.Get("server_addr"),
		"server_name":          q.Get("server_name"),
		"mtu":                  q.Get("mtu"),
		"ca_pem":               string(caCertPEM),
		"cert_pem":             string(clientCertPEM),
		"key_pem":              string(clientKeyPEM),
		"key_log_file":         q.Get("key_log_file"),
		"log_level":            q.Get("log_level"),
		"insecure_skip_verify": q.Get("insecure_skip_verify"),
		"tun_name":             q.Get("tun_name"),
	}
	// 默认值处理
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
	// 替换所有 {{key}}
	for k, v := range repl {
		tmpl = strings.ReplaceAll(tmpl, "{{"+k+"}}", v)
	}
	config := tmpl

	// 写入数据库
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		http.Error(w, "数据库错误", 500)
		return
	}
	defer db.Close()
	_, err = db.Exec("INSERT INTO clients(client_id, cert_pem, key_pem, config, created_at) VALUES (?, ?, ?, ?, datetime('now'))",
		clientID, string(clientCertPEM), string(clientKeyPEM), config)
	if err != nil {
		http.Error(w, "写入数据库失败", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"client_id": clientID})
}

// 下载客户端配置
func handleDownloadClient(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "缺少id参数", 400)
		return
	}
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		http.Error(w, "数据库错误", 500)
		return
	}
	defer db.Close()
	var config string
	err = db.QueryRow("SELECT config FROM clients WHERE client_id = ?", id).Scan(&config)
	if err != nil {
		http.Error(w, "未找到该客户端", 404)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=config.client.toml")
	w.Write([]byte(config))
}

// 删除客户端
// 修正：ipConnMap 类型为 map[netip.Addr]*connectip.Conn
func handleDeleteClient(ipPoolMu *sync.Mutex, clientIPMap map[string]netip.Addr, ipConnMap map[netip.Addr]*connectip.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "缺少id参数", 400)
			return
		}
		// 主动断开在线连接
		// 修正：确保 ipPoolMu, clientIPMap, ipConnMap 不为 nil
		if ipPoolMu != nil && clientIPMap != nil && ipConnMap != nil {
			ipPoolMu.Lock()
			// 直接查找 client_id 对应 IP 并断开
			if ip, ok := clientIPMap[id]; ok {
				if conn, ok2 := ipConnMap[ip]; ok2 {
					// 修正：直接调用 Close()，无需类型断言
					log.Printf("主动断开客户端 %s (IP: %s) 的连接", id, ip)
					conn.Close() // 直接调用
					delete(ipConnMap, ip)
				}
				delete(clientIPMap, id)
				// 注意：IP 地址的释放应该由 handleClientConnection 的 defer 逻辑处理，这里只删除映射关系和断开连接
			}
			ipPoolMu.Unlock()
		}
		// 删除数据库记录
		db, err := sql.Open("sqlite3", dbFile)
		if err != nil {
			http.Error(w, "数据库错误", 500)
			return
		}
		defer db.Close()
		_, err = db.Exec("DELETE FROM clients WHERE client_id = ?", id)
		if err != nil {
			http.Error(w, "删除失败", 500)
			return
		}
		w.Write([]byte("ok"))
	}
}

// 修正：ipConnMap 类型为 map[netip.Addr]*connectip.Conn
func StartAPIServer(ipPoolMu *sync.Mutex, clientIPMap map[string]netip.Addr, ipConnMap map[netip.Addr]*connectip.Conn) {
	initDB()
	http.Handle("/", http.FileServer(http.Dir("./web")))
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/ca_status", requireAuth(handleCAStatus))
	http.HandleFunc("/api/clients", requireAuth(handleListClients(clientIPMap)))
	http.HandleFunc("/api/gen_client", requireAuth(handleGenClientV2))
	http.HandleFunc("/api/download_client", requireAuth(handleDownloadClient))
	http.HandleFunc("/api/delete_client", requireAuth(handleDeleteClient(ipPoolMu, clientIPMap, ipConnMap)))
	// 新增服务器配置API
	http.HandleFunc("/api/server_config", requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleGetServerConfig(w, r)
		} else if r.Method == http.MethodPost {
			handleSetServerConfig(w, r)
		} else {
			http.Error(w, "不支持的方法", 405)
		}
	}))
	log.Println("API server listening on 0.0.0.0:8080 ...")
	_ = http.ListenAndServe("0.0.0.0:8080", nil)
}

// ====== 服务器配置API ======
type ServerConfigDB struct {
	ServerAddr string `json:"server_addr"`
	ServerName string `json:"server_name"`
	MTU        int    `json:"mtu"`
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

func handleGetServerConfig(w http.ResponseWriter, r *http.Request) {
	_ = r
	cfg, err := getServerConfigFromDB()
	if err != nil {
		// 首次无数据，返回默认
		cfg = ServerConfigDB{ServerAddr: "", ServerName: "", MTU: 1413}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

func handleSetServerConfig(w http.ResponseWriter, r *http.Request) {
	var req ServerConfigDB
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "参数错误", 400)
		return
	}
	if req.MTU < 576 || req.MTU > 9000 {
		http.Error(w, "MTU不合法", 400)
		return
	}
	if err := saveServerConfigToDB(req); err != nil {
		http.Error(w, "保存失败", 500)
		return
	}
	w.Write([]byte("ok"))
}
