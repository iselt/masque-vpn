# masque-vpn

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/iselt/masque-vpn)

A VPN implementation based on the MASQUE(CONNECT-IP) protocol

**⚠ This project is in early development and is not ready for production use. It is intended for educational purposes and to demonstrate the MASQUE protocol.**

## Build

You can use the Makefile (recommended) or build manually:

```bash
# Build both client and server (cross-platform)
make all
# Or build individually
cd vpn_client && go build
cd ../vpn_server && go build
```

## Certificate Generation

Before running, generate CA, server, and client certificates:

```bash
cd vpn_server/cert
# Generate CA
sh gen_ca.sh
# Generate server certificate
sh gen_server_keypair.sh
# Generate client certificate (optional, usually via Web UI)
sh gen_client_keypair.sh
```
On Windows, use Git Bash/WSL or run the openssl commands in the scripts manually.

## Configuration

- Copy `vpn_server/config.server.toml.example` to `vpn_server/config.server.toml` and edit as needed.
- Client config can be generated via the Web UI (recommended).

## Start

Start the server first, then the client:

```bash
cd vpn_server
./vpn-server
```

```bash
cd vpn_client
./vpn-client
```

## Web Management UI

- After starting the server, visit: `http://<server-ip>:8080/`
- Default admin account: `admin` / `admin`
- You can generate/download client configs, manage online clients, and delete clients via the Web UI.

## References

This project is built upon the following open-source libraries:

* [quic-go](https://github.com/quic-go/quic-go) - A QUIC implementation in Go
* [connect-ip-go](https://github.com/quic-go/connect-ip-go) - A Go implementation of the MASQUE CONNECT-IP protocol

## 中文文档

请参考 [README_zh.md](README_zh.md) 获取中文使用说明。