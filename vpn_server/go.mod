module vpn-server

go 1.24.2

require (
	github.com/BurntSushi/toml v1.5.0
	github.com/iselt/masque-vpn/common_utils v0.0.0-00010101000000-000000000000
	github.com/quic-go/connect-ip-go v0.0.0-20241112091351-321f13c3d203
	github.com/quic-go/quic-go v0.50.1
	github.com/yosida95/uritemplate/v3 v3.0.2
)

require (
	github.com/dunglas/httpsfv v1.1.0 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/google/pprof v0.0.0-20250403155104-27863c87afa6 // indirect
	github.com/onsi/ginkgo/v2 v2.23.4 // indirect
	github.com/quic-go/qpack v0.5.1 // indirect
	github.com/vishvananda/netlink v1.3.0 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.uber.org/mock v0.5.1 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/tools v0.32.0 // indirect
	golang.zx2c4.com/wintun v0.0.0-20230126152724-0fa3db229ce2 // indirect
	golang.zx2c4.com/wireguard v0.0.0-20231211153847-12269c276173 // indirect
	golang.zx2c4.com/wireguard/windows v0.5.3 // indirect
)

replace github.com/iselt/masque-vpn/common_utils => ../common_utils
