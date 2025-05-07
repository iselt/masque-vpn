package common

// ClientConfig 结构体，用于存储从 TOML 文件加载的客户端配置信息
// 可供 vpn_client/main.go 使用
//
type ClientConfig struct {
	ServerAddr         string `toml:"server_addr"`
	ServerName         string `toml:"server_name"`
	CAFile             string `toml:"ca_file"`
	CAPEM              string `toml:"ca_pem"`
	TLSCert            string `toml:"tls_cert"`
	TLSKey             string `toml:"tls_key"`
	CertPEM            string `toml:"cert_pem"`
	KeyPEM             string `toml:"key_pem"`
	InsecureSkipVerify bool   `toml:"insecure_skip_verify"`
	TunName            string `toml:"tun_name"`
	KeyLogFile         string `toml:"key_log_file"`
	LogLevel           string `toml:"log_level"`
	MTU                int    `toml:"mtu"`
}

// ServerConfig 结构体，用于存储从 TOML 文件加载的服务端配置信息
// 可供 vpn_server/main.go 使用
//
type ServerConfig struct {
	ListenAddr      string   `toml:"listen_addr"`
	CertFile        string   `toml:"cert_file"`
	KeyFile         string   `toml:"key_file"`
	CACertFile      string   `toml:"ca_cert_file"`
	CAKeyFile       string   `toml:"ca_key_file"`
	CertPEM         string   `toml:"cert_pem"`
	KeyPEM          string   `toml:"key_pem"`
	CAKeyPEM        string   `toml:"ca_key_pem"`
	CACertPEM       string   `toml:"ca_cert_pem"`
	AssignCIDR      string   `toml:"assign_cidr"`
	AdvertiseRoutes []string `toml:"advertise_routes"`
	TunName         string   `toml:"tun_name"`
	LogLevel        string   `toml:"log_level"`
	ServerName      string   `toml:"server_name"`
	MTU             int      `toml:"mtu"`

	// 新增：API服务器监听地址
	APIServerListenAddr string `toml:"api_server.listen_addr"`
}
