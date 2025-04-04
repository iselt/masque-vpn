# masque-vpn

A VPN implementation based on the MASQUE(CONNECT-IP) protocol

## generate certificates for server

```bash
openssl req -x509 -newkey rsa:4096 -nodes \
  -keyout server.key.pem -out server.crt.pem \
  -sha256 -days 365 \
  -subj "/CN=vpn.example.com" \
  -addext "subjectAltName=DNS:vpn.example.com,IP:127.0.0.1"
```