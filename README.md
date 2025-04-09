# masque-vpn

A VPN implementation based on the MASQUE(CONNECT-IP) protocol

**⚠ This project is in early development and is not ready for production use. It is intended for educational purposes and to demonstrate the MASQUE protocol.**

## Usage

### Build

```bash
cd vpn_client
go build
cd ../vpn_server
go build
```

### Configure

#### Client

```bash
cp config.client.toml.example config.client.toml
```

Edit `config.client.toml` accrording to your needs.

#### Server

Generate certificates for server

```bash
openssl req -x509 -newkey rsa:4096 -nodes \
  -keyout server.key.pem -out server.crt.pem \
  -sha256 -days 365 \
  -subj "/CN=vpn.example.com" \
  -addext "subjectAltName=DNS:vpn.example.com,IP:127.0.0.1"
```

```bash
cp config.server.toml.example config.server.toml
```

Edit `config.server.toml` accrording to your needs.

### Start

#### Server

```bash
sudo ./vpn-server
```

#### Client

```bash
sudo ./vpn-client
```