# masque-vpn

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/iselt/masque-vpn)

基于 MASQUE (CONNECT-IP) 协议的 VPN 实现。

**⚠️ 本项目处于早期开发阶段，暂不适合生产环境使用，仅供学习和 MASQUE 协议演示用途。**

**本项目包含了一个子模块 `iselt/connect-ip-go`, 请使用 `--recurse-submodules` 克隆项目：`git clone --recurse-submodules https://github.com/iselt/masque-vpn.git`**

## 特性

- **现代协议**: 基于 QUIC 和 MASQUE CONNECT-IP 构建
- **双向 TLS 认证**: 基于证书的客户端-服务器认证
- **Web 管理界面**: 基于浏览器的客户端管理和配置
- **跨平台支持**: 支持 Windows、Linux 和 macOS
- **IP 池管理**: 自动客户端 IP 分配和路由
- **实时监控**: 实时客户端连接状态

## 架构

系统组成：
- **VPN 服务器**: 处理客户端连接和流量路由
- **VPN 客户端**: 连接服务器并路由本地流量
- **Web 界面**: 证书和客户端管理界面
- **证书系统**: 基于 PKI 的双向 TLS 认证

## 快速开始

### 1. 编译

```bash
cd vpn_client && go build
cd ../vpn_server && go build
cd ../admin_webui && npm install && npm run build
```

### 2. 证书设置

```bash
cd vpn_server/cert
# 生成 CA 证书
sh gen_ca.sh
# 生成服务器证书
sh gen_server_keypair.sh
```

### 3. 服务器配置

复制并编辑服务器配置：
```bash
cp vpn_server/config.server.toml.example vpn_server/config.server.toml
```

### 4. 启动服务器

```bash
cd vpn_server
./vpn-server
```

### 5. Web 管理

- 访问地址: `http://<服务器IP>:8080/`
- 默认账号: `admin` / `admin`
- 通过 Web 界面生成客户端配置

### 6. 启动客户端

```bash
cd vpn_client
./vpn-client
```

## 配置说明

### 服务器配置

`config.server.toml` 中的主要配置选项：

| 选项 | 说明 | 示例 |
|------|------|------|
| `listen_addr` | 服务器监听地址 | `"0.0.0.0:4433"` |
| `assign_cidr` | 客户端 IP 范围 | `"10.0.0.0/24"` |
| `advertise_routes` | 广播路由 | `["0.0.0.0/0"]` |
| `cert_file` | 服务器证书路径 | `"cert/server.crt"` |
| `key_file` | 服务器私钥路径 | `"cert/server.key"` |

### 客户端配置

通过 Web 界面自动生成或手动配置：

| 选项 | 说明 |
|------|------|
| `server_addr` | VPN 服务器地址 |
| `server_name` | TLS 服务器名称 |
| `ca_pem` | CA 证书（内嵌） |
| `cert_pem` | 客户端证书（内嵌） |
| `key_pem` | 客户端私钥（内嵌） |

## Web 管理界面

Web 界面提供：

- **客户端管理**: 生成、下载和删除客户端配置
- **实时监控**: 查看已连接客户端及其 IP 分配
- **证书管理**: 自动化证书生成和分发
- **配置管理**: 服务器设置管理

## 技术细节

### 依赖项

- **QUIC**: [quic-go](https://github.com/quic-go/quic-go) - QUIC 协议实现
- **MASQUE**: [connect-ip-go](https://github.com/quic-go/connect-ip-go) - MASQUE CONNECT-IP 协议
- **数据库**: SQLite 用于客户端和配置存储
- **TUN**: 跨平台 TUN 设备管理

### 安全性

- **双向 TLS**: 客户端和服务器使用证书相互认证
- **证书颁发机构**: 自签名 CA 用于证书管理
- **唯一客户端 ID**: 每个客户端都有唯一标识符
- **IP 隔离**: 客户端接收独立的 IP 分配

## 开发

### 项目结构

```
masque-vpn/
├── common/           # 共享代码和工具
├── vpn_client/       # 客户端实现
├── vpn_server/       # 服务器实现
│   └── cert/         # 证书生成脚本
├── admin_webui/      # Web 界面资源
└── README_zh.md
```

### 从源码构建

要求：
- Go 1.24.2 或更高版本
- OpenSSL（用于证书生成）

## 故障排除

### 常见问题

1. **证书错误**: 确保 CA 和证书正确生成
2. **权限问题**: TUN 设备创建需要管理员权限
3. **防火墙**: 确保服务器端口（默认 4433）可访问
4. **MTU 问题**: 如遇连接问题，请调整 MTU 设置

## 贡献

本项目用于教育目的。欢迎贡献：
- 协议改进
- 跨平台兼容性
- 文档完善
- 错误修复

## 参考资料

- [MASQUE 协议规范](https://datatracker.ietf.org/doc/draft-ietf-masque-connect-ip/)
- [QUIC 协议](https://datatracker.ietf.org/doc/rfc9000/)
- [quic-go 库](https://github.com/quic-go/quic-go)
- [connect-ip-go 库](https://github.com/quic-go/connect-ip-go)