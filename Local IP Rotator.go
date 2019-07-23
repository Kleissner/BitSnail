package main

import (
	"context"
	"net"
)

// nextLocalIP is the next available local IP address to use. Only IPv4!
var nextLocalIP = "127.0.0.3"

// getLocalIP returns a local IP address to use.
// Using multiple IP addresses in different connections can help against local port exhaustion.
// It will return a consecutive one from 127.0.0.2/8.
func getLocalIP() (ip net.IP) {
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
type direct2 struct{}

// DialerIPRotate implements Dialer by making network connections directly using net.Dial or net.DialContext.
// It replaces proxy.Direct.
var DialerIPRotate = direct2{}

// Dial directly invokes net.Dial with the supplied parameters.
func (direct2) Dial(network, addr string) (net.Conn, error) {
	localAddr := &net.TCPAddr{IP: getLocalIP(), Zone: ""}

	var d net.Dialer
	d.LocalAddr = localAddr
	return d.Dial(network, addr)
}

// DialContext instantiates a net.Dialer and invokes its DialContext receiver with the supplied parameters.
func (direct2) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	localAddr := &net.TCPAddr{IP: getLocalIP(), Zone: ""}

	var d net.Dialer
	d.LocalAddr = localAddr
	return d.DialContext(ctx, network, addr)
}
