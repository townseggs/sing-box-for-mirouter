//go:build router

package vless

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-vmess/vless"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.VLESSOutboundOptions](registry, C.TypeVLESS, NewOutbound)
}

type Outbound struct {
	outbound.Adapter
	logger     logger.ContextLogger
	dialer     N.Dialer
	client     *vless.Client
	serverAddr M.Socksaddr
	tlsConfig  tls.Config
	xudp       bool
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.VLESSOutboundOptions) (adapter.Outbound, error) {
	// The router profile keeps raw TCP with TLS/REALITY only.
	// WebSocket, HTTP, HTTPUpgrade, gRPC and QUIC transports are not linked.
	if options.Transport != nil {
		return nil, E.New("transport is not included in the router build")
	}
	// Vision (flow=xtls-rprx-vision) is incompatible with multiplexing.
	// See https://github.com/SagerNet/sing-box/issues/1535#issuecomment-1977952414
	if options.Multiplex != nil {
		return nil, E.New("multiplex is not included in the router build")
	}
	outboundDialer, err := dialer.New(ctx, options.DialerOptions, options.ServerIsDomain())
	if err != nil {
		return nil, err
	}
	instance := &Outbound{
		Adapter:    outbound.NewAdapterWithDialerOptions(C.TypeVLESS, tag, options.Network.Build(), options.DialerOptions),
		logger:     logger,
		dialer:     outboundDialer,
		serverAddr: options.ServerOptions.Build(),
	}
	if options.TLS != nil {
		instance.tlsConfig, err = tls.NewClient(ctx, options.Server, *options.TLS)
		if err != nil {
			return nil, err
		}
	}
	if options.PacketEncoding == nil {
		instance.xudp = true
	} else {
		// The router profile omits packetaddr and supports only empty or xudp.
		switch *options.PacketEncoding {
		case "":
		case "xudp":
			instance.xudp = true
		default:
			return nil, E.New("unknown packet encoding: ", *options.PacketEncoding)
		}
	}
	instance.client, err = vless.NewClient(options.UUID, options.Flow, logger)
	if err != nil {
		return nil, err
	}
	return instance, nil
}

func (h *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
	case N.NetworkUDP:
		h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	}
	return h.dial(ctx, network, destination)
}

func (h *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	return h.listenPacket(ctx, destination)
}

func (h *Outbound) InterfaceUpdated() {}

func (h *Outbound) Close() error {
	return nil
}

func (h *Outbound) dial(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	conn, err := h.openConn(ctx, destination)
	if err != nil {
		return nil, err
	}
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		return h.client.DialEarlyConn(conn, destination)
	case N.NetworkUDP:
		if h.xudp {
			return h.client.DialEarlyXUDPPacketConn(conn, destination)
		}
		return h.client.DialEarlyPacketConn(conn, destination)
	default:
		conn.Close()
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (h *Outbound) listenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	conn, err := h.openConn(ctx, destination)
	if err != nil {
		return nil, err
	}
	if h.xudp {
		return h.client.DialEarlyXUDPPacketConn(conn, destination)
	}
	return h.client.DialEarlyPacketConn(conn, destination)
}

func (h *Outbound) openConn(ctx context.Context, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	conn, err := h.dialer.DialContext(ctx, N.NetworkTCP, h.serverAddr)
	if err != nil {
		return nil, err
	}
	if h.tlsConfig != nil {
		tlsConn, err := tls.ClientHandshake(ctx, conn, h.tlsConfig)
		if err != nil {
			conn.Close()
			return nil, err
		}
		conn = tlsConn
	}
	return conn, nil
}
