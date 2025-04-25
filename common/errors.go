package common

import "errors"

// 通用错误定义，可在 client/server 复用
var (
	ErrInvalidConfig    = errors.New("配置文件无效或缺少必要字段")
	ErrTunDeviceCreate  = errors.New("TUN 设备创建失败")
	ErrQuicDial         = errors.New("QUIC 连接建立失败")
	ErrConnectIP        = errors.New("CONNECT-IP 连接失败")
	ErrNoAssignedPrefix = errors.New("未分配到网络前缀")
)
