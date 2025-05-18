# Makefile

# Go 源码目录
CLIENT_DIR=./vpn_client
SERVER_DIR=./vpn_server
WEBUI_DIR=./admin_webui

# 输出文件名
CLIENT_BIN= vpn-client
SERVER_BIN= vpn-server

# 默认目标
.PHONY: all
all: build-client build-server build-webui

# 分别编译 client
.PHONY: build-client
build-client: build-client-win build-client-linux

.PHONY: build-client-win
build-client-win:
	cd $(CLIENT_DIR) && GOOS=windows GOARCH=amd64 go build

.PHONY: build-client-linux
build-client-linux:
	cd $(CLIENT_DIR) && GOOS=linux GOARCH=amd64 go build

# 分别编译 server（加 CGO_ENABLED=1）
.PHONY: build-server
build-server: build-server-linux

.PHONY: build-server-win
build-server-win:
	cd $(SERVER_DIR) && CC=x86_64-w64-mingw32-gcc && CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build

.PHONY: build-server-linux
build-server-linux:
	cd $(SERVER_DIR) && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build

# 编译 Web UI
.PHONY: build-webui
build-webui:
	cd $(WEBUI_DIR) && npm install && npm run build

# 清理
.PHONY: clean
clean:
	rm -f $(CLIENT_DIR)/vpn-client $(CLIENT_DIR)/vpn-client.exe
	rm -f $(SERVER_DIR)/vpn-server $(SERVER_DIR)/vpn-server.exe
	cd $(WEBUI_DIR) && rm -rf dist node_modules