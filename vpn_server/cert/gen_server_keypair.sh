openssl genrsa -out server.key 4096
openssl req -new -key server.key -out server.csr -subj "/CN=vpn.example.local"
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 365 -extensions v3_req -extfile openssl-san.cnf