# Makefile

# Go 源码目录
CLIENT_DIR=./vpn_client
SERVER_DIR=./vpn_server

# 输出文件名
CLIENT_BIN= vpn-client
SERVER_BIN= vpn-server

# 默认目标
.PHONY: all
all: build-client build-server

# 分别编译 client
.PHONY: build-client
ifeq ($(OS),Windows_NT)
build-client:
	cd $(CLIENT_DIR) && go build -o vpn-client main.go
else
build-client:
	cd $(CLIENT_DIR) && go build -o vpn-client main.go
endif

.PHONY: build-client-win
ifeq ($(OS),Windows_NT)
build-client-win:
	cd $(CLIENT_DIR) && set GOOS=windows && set GOARCH=amd64 && go build -o vpn-client.exe main.go
else
build-client-win:
	cd $(CLIENT_DIR) && GOOS=windows GOARCH=amd64 go build -o vpn-client.exe main.go
endif

.PHONY: build-client-linux
ifeq ($(OS),Windows_NT)
build-client-linux:
	cd $(CLIENT_DIR) && set GOOS=linux && set GOARCH=amd64 && go build -o vpn-client main.go
else
build-client-linux:
	cd $(CLIENT_DIR) && GOOS=linux GOARCH=amd64 go build -o vpn-client main.go
endif

# 分别编译 server（加 CGO_ENABLED=1）
.PHONY: build-server
ifeq ($(OS),Windows_NT)
build-server:
	cd $(SERVER_DIR) && set CGO_ENABLED=1 && go build
else
build-server:
	cd $(SERVER_DIR) && CGO_ENABLED=1 go build
endif

.PHONY: build-server-win
ifeq ($(OS),Windows_NT)
build-server-win:
	cd $(SERVER_DIR) && set CGO_ENABLED=1 && set GOOS=windows && set GOARCH=amd64 && go build
else
build-server-win:
	cd $(SERVER_DIR) && CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build
endif

.PHONY: build-server-linux
ifeq ($(OS),Windows_NT)
build-server-linux:
	cd $(SERVER_DIR) && set CGO_ENABLED=1 && set GOOS=linux && set GOARCH=amd64 && go build
else
build-server-linux:
	cd $(SERVER_DIR) && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build
endif

# 合并编译命令
.PHONY: build-all
build-all: build-client-linux build-client-win build-server-linux build-server-win

# 清理
.PHONY: clean
ifeq ($(OS),Windows_NT)
clean:
	powershell -Command "Remove-Item -Force -ErrorAction SilentlyContinue $(CLIENT_DIR)\\vpn-client, $(CLIENT_DIR)\\vpn-client.exe, $(SERVER_DIR)\\vpn-server, $(SERVER_DIR)\\vpn-server.exe"
else
clean:
	rm -f $(CLIENT_DIR)/vpn-client $(CLIENT_DIR)/vpn-client.exe
	rm -f $(SERVER_DIR)/vpn-server $(SERVER_DIR)/vpn-server.exe
endif