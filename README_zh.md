# masque-vpn

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/iselt/masque-vpn)

基于 MASQUE(CONNECT-IP) 协议的 VPN 实现

**⚠ 本项目处于早期开发阶段，暂不适合生产环境，仅供学习和 MASQUE 协议演示用途。**

## 编译

推荐直接使用 Makefile：

```bash
make all
```

或手动编译：

```bash
cd vpn_client && go build
cd ../vpn_server && go build
```

## 证书生成

首次运行前需生成 CA、服务端和客户端证书：

```bash
cd vpn_server/cert
# 生成 CA
sh gen_ca.sh
# 生成服务端证书
sh gen_server_keypair.sh
# 生成客户端证书（可选，推荐用 Web 管理后台自动生成）
sh gen_client_keypair.sh
```
Windows 下建议用 Git Bash/WSL，或手动执行 openssl 命令。

## 配置

- 复制 `vpn_server/config.server.toml.example` 为 `vpn_server/config.server.toml` 并按需修改。
- 客户端配置建议通过 Web 管理后台自动生成。

## 启动

先启动服务端，再启动客户端：

```bash
cd vpn_server
./vpn-server
```

```bash
cd vpn_client
./vpn-client
```

## Web 管理后台

- 启动服务端后，访问 `http://服务器IP:8080/`
- 默认管理员账号：admin，密码：admin
- 可在 Web UI 一键生成/下载客户端配置，管理在线客户端，删除客户端等。

## 参考

本项目基于以下开源库：

* [quic-go](https://github.com/quic-go/quic-go) - Go 语言实现的 QUIC 协议
* [connect-ip-go](https://github.com/quic-go/connect-ip-go) - Go 语言实现的 MASQUE CONNECT-IP 协议
