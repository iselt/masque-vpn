# masque-vpn

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/iselt/masque-vpn)

A VPN implementation based on the MASQUE (CONNECT-IP) protocol using QUIC transport.

**⚠️ This project is in early development and is not ready for production use. It is intended for educational purposes and to demonstrate the MASQUE protocol.**

## Features

- **Modern Protocols**: Built on QUIC and MASQUE CONNECT-IP
- **Mutual TLS Authentication**: Certificate-based client-server authentication
- **Web Management UI**: Browser-based client management and configuration
- **Cross-Platform**: Supports Windows, Linux, and macOS
- **IP Pool Management**: Automatic client IP allocation and routing
- **Real-time Monitoring**: Live client connection status

## Architecture

The system consists of:
- **VPN Server**: Handles client connections and traffic routing
- **VPN Client**: Connects to server and routes local traffic
- **Web UI**: Management interface for certificates and clients
- **Certificate System**: PKI-based authentication using mutual TLS

## Quick Start

### 1. Build

```bash
cd vpn_client && go build
cd ../vpn_server && go build
cd ../admin_webui && npm install && npm run build
```

### 2. Certificate Setup

```bash
cd vpn_server/cert
# Generate CA certificate
sh gen_ca.sh
# Generate server certificate
sh gen_server_keypair.sh
```

### 3. Server Configuration

Copy and edit the server configuration:
```bash
cp vpn_server/config.server.toml.example vpn_server/config.server.toml
```

### 4. Start Server

```bash
cd vpn_server
./vpn-server
```

### 5. Web Management

- Access: `http://<server-ip>:8080/`
- Default credentials: `admin` / `admin`
- Generate client configurations through the web interface

### 6. Start Client

```bash
cd vpn_client
./vpn-client
```

## Configuration

### Server Configuration

Key configuration options in `config.server.toml`:

| Option | Description | Example |
|--------|-------------|---------|
| `listen_addr` | Server listening address | `"0.0.0.0:4433"` |
| `assign_cidr` | IP range for clients | `"10.0.0.0/24"` |
| `advertise_routes` | Routes to advertise | `["0.0.0.0/0"]` |
| `cert_file` | Server certificate path | `"cert/server.crt"` |
| `key_file` | Server private key path | `"cert/server.key"` |

### Client Configuration

Generated automatically via Web UI or manually configured:

| Option | Description |
|--------|-------------|
| `server_addr` | VPN server address |
| `server_name` | Server name for TLS |
| `ca_pem` | CA certificate (embedded) |
| `cert_pem` | Client certificate (embedded) |
| `key_pem` | Client private key (embedded) |

## Web Management Interface

The web interface provides:

- **Client Management**: Generate, download, and delete client configurations
- **Live Monitoring**: View connected clients and their IP assignments
- **Certificate Management**: Automated certificate generation and distribution
- **Configuration**: Server settings management

## Technical Details

### Dependencies

- **QUIC**: [quic-go](https://github.com/quic-go/quic-go) - QUIC protocol implementation
- **MASQUE**: [connect-ip-go](https://github.com/quic-go/connect-ip-go) - MASQUE CONNECT-IP protocol
- **Database**: SQLite for client and configuration storage
- **TUN**: Cross-platform TUN device management

### Security

- **Mutual TLS**: Both client and server authenticate using certificates
- **Certificate Authority**: Self-signed CA for certificate management
- **Unique Client IDs**: Each client has a unique identifier
- **IP Isolation**: Clients receive individual IP assignments

## Development

### Project Structure

```
masque-vpn/
├── common/           # Shared code and utilities
├── vpn_client/       # Client implementation
├── vpn_server/       # Server implementation
│   └── cert/         # Certificate generation scripts
├── admin_webui/      # Web UI assets
└── README.md
```

### Building from Source

Requirements:
- Go 1.24.2 or later
- OpenSSL (for certificate generation)

## Troubleshooting

### Common Issues

1. **Certificate Errors**: Ensure CA and certificates are properly generated
2. **Permission Issues**: TUN device creation requires administrator privileges
3. **Firewall**: Ensure server port (default 4433) is accessible
4. **MTU Issues**: Adjust MTU settings if experiencing connectivity problems

## Contributing

This project is for educational purposes. Contributions are welcome for:
- Protocol improvements
- Cross-platform compatibility
- Documentation enhancements
- Bug fixes

## References

- [MASQUE Protocol Specification](https://datatracker.ietf.org/doc/draft-ietf-masque-connect-ip/)
- [QUIC Protocol](https://datatracker.ietf.org/doc/rfc9000/)
- [quic-go Library](https://github.com/quic-go/quic-go)
- [connect-ip-go Library](https://github.com/quic-go/connect-ip-go)


This project is built upon the following open-source libraries:

* [quic-go](https://github.com/quic-go/quic-go) - A QUIC implementation in Go
* [connect-ip-go](https://github.com/quic-go/connect-ip-go) - A Go implementation of the MASQUE CONNECT-IP protocol

## 中文文档

请参考 [README_zh.md](README_zh.md) 获取中文使用说明。