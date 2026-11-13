//go:build router

// 路由器裁剪版注册表（build tag：router）。
//
// 只注册两台路由器实际用到的模块：
//
//	inbound     redirect（iptables REDIRECT 透明代理）
//	            direct（DNS 入口 5053 / QUIC 入口 2443）
//	outbound    vless（VLESS + REALITY 出站，TLS/uTLS 由 with_utls tag 提供）
//	            direct
//	DNS 传输    udp、tcp、hosts、local
//	rule-set    远程 .srs 由路由层自行处理，不需要注册
//
// 要继续裁就改这里；要加回某个协议也是一行的事。
//
// 构建：
//
//	go build -tags "with_utls,router" -trimpath -ldflags "-s -w" -o sing-box ./cmd/sing-box   # 本文件（裁剪版）
//	go build -tags "with_utls"        -trimpath -ldflags "-s -w" -o sing-box ./cmd/sing-box   # 上游全量版（对照）
//
// 注意：本文件与 include/registry.go 是互斥的两个版本，两者必须提供完全相同的
// 函数集合（Context / InboundRegistry / OutboundRegistry / EndpointRegistry /
// DNSTransportRegistry / ServiceRegistry）。
package include

import (
	"context"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/dns/transport/hosts"
	"github.com/sagernet/sing-box/dns/transport/local"
	"github.com/sagernet/sing-box/protocol/direct"
	"github.com/sagernet/sing-box/protocol/redirect"
	"github.com/sagernet/sing-box/protocol/vless"
)

func Context(ctx context.Context) context.Context {
	return box.Context(ctx, InboundRegistry(), OutboundRegistry(), EndpointRegistry(), DNSTransportRegistry(), ServiceRegistry())
}

func InboundRegistry() *inbound.Registry {
	registry := inbound.NewRegistry()

	redirect.RegisterRedirect(registry)
	direct.RegisterInbound(registry)

	return registry
}

func OutboundRegistry() *outbound.Registry {
	registry := outbound.NewRegistry()

	direct.RegisterOutbound(registry)
	vless.RegisterOutbound(registry)

	return registry
}

// 不用 wireguard / tailscale endpoint。
func EndpointRegistry() *endpoint.Registry {
	return endpoint.NewRegistry()
}

func DNSTransportRegistry() *dns.TransportRegistry {
	registry := dns.NewTransportRegistry()

	transport.RegisterUDP(registry)
	transport.RegisterTCP(registry)
	hosts.RegisterTransport(registry)
	local.RegisterTransport(registry)

	return registry
}

// 配置里没有 services 段。
func ServiceRegistry() *service.Registry {
	return service.NewRegistry()
}
