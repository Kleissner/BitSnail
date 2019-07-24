package main

import (
	"context"
	"net"

	"golang.org/x/net/proxy"
)

// nextLocalIP is the next available local IP address to use. Only IPv4!
var nextLocalIP = "127.0.0.3"

// getLoopbackIP returns a local IP address to use.
// Using multiple IP addresses in different connections can help against local port exhaustion.
// It will return a consecutive one from 127.0.0.2/8.
func getLoopbackIP() (ip net.IP) {
	ip = net.ParseIP(nextLocalIP)

	// rotate and then assign back
	nextLocalIP = nextIP(ip, 1).String()

	return ip
}

func nextIP(ip net.IP, inc uint) net.IP {
	i := ip.To4()
	v := uint(i[0])<<24 + uint(i[1])<<16 + uint(i[2])<<8 + uint(i[3])
	v += inc
	v3 := byte(v & 0xFF)
	v2 := byte((v >> 8) & 0xFF)
	v1 := byte((v >> 16) & 0xFF)
	v0 := byte((v >> 24) & 0xFF)
	return net.IPv4(v0, v1, v2, v3)
}

// Dialer to bind to IP
type direct2 struct {
	localIPs []string
	rotate   int
}

// GetDialerBindLocalIP creates a dialer to use for binding connections to a proxy to a local IP client side.
// If no LocalIPs are supplied, it will use a rotated loopback IP.
// This is a replacement for proxy.Direct.
// Note: You must not use loopback IPs with connections to a proxy that is listening on a non-local address, as it would be not routable.
func GetDialerBindLocalIP(LocalIPs []string) proxy.Dialer {
	if len(LocalIPs) == 0 {
		return direct2{}
	}

	return direct2{localIPs: LocalIPs}
}

func (d2 direct2) getLocalIP() (ip net.IP) {
	if len(d2.localIPs) > 0 {
		ip := d2.localIPs[d2.rotate%len(d2.localIPs)]
		d2.rotate++
		return net.ParseIP(ip)
	}

	return getLoopbackIP()
}

// Dial directly invokes net.Dial with the supplied parameters.
func (d2 direct2) Dial(network, addr string) (net.Conn, error) {
	localAddr := &net.TCPAddr{IP: d2.getLocalIP(), Zone: ""}

	var d net.Dialer
	d.LocalAddr = localAddr
	return d.Dial(network, addr)
}

// DialContext instantiates a net.Dialer and invokes its DialContext receiver with the supplied parameters.
func (d2 direct2) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	localAddr := &net.TCPAddr{IP: d2.getLocalIP(), Zone: ""}

	var d net.Dialer
	d.LocalAddr = localAddr
	return d.DialContext(ctx, network, addr)
}
