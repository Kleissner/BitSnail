// Proxy rotation code for maintaining a list of SOCKS5 proxies.
package main

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

var proxies []*ProxyInfo

// ProxyInfo contains information of one proxy
type ProxyInfo struct {
	address string
	active  bool
	dialer  proxy.Dialer
}

// ProxyAdd adds a proxy to the list.
func ProxyAdd(address string, active bool, dialer proxy.Dialer) {
	proxies = append(proxies, &ProxyInfo{address: address, active: active})
}

// ProxyAddVerify adds a proxy if it's active. Otherwise it drops it immediately.
// Call this function as Go routine as it may stall!
// Future: The proxy could be added to an invalid list? For re-checking?
// Rechecking proxies that are already valid is a dangerous thing; the ddos attack may slow them down
// and unstable - so checking them while ddosing would result in false positives.
func ProxyAddVerify(address string, dialer proxy.Dialer) {
	timeStart := time.Now()
	valid, _ := checkProxySOCKS5(address, dialer)
	if valid {
		fmt.Printf("Adding valid proxy %s (response time %s)\n", address, time.Since(timeStart).String())
		ProxyAdd(address, true, dialer)
		return
	}

	fmt.Printf("Rejecting invalid proxy %s\n", address)
}

var proxyRotation int

// ProxyGet gets an available proxy
func ProxyGet() *ProxyInfo {
	if len(proxies) == 0 {
		return nil
	}

	for n := 0; n < len(proxies); n++ {
		if entry := proxies[proxyRotation%len(proxies)]; entry.active {
			proxyRotation++
			return entry
		}

		proxyRotation++
	}

	return nil
}

// checkProxySOCKS5 checks if a proxy is alive. Note that this function currently does not support IP binding.
// Use this function in a Go routine, as it may stall.
func checkProxySOCKS5(address string, dialer proxy.Dialer) (valid bool, err error) {
	dialerP, _ := proxy.SOCKS5("tcp", address, nil, dialer)

	timeout := time.Duration(5 * time.Second)
	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			Dial:              dialerP.Dial,
		},
	}

	response, err := httpClient.Get("https://api.ipify.org")

	if err != nil {
		return false, err
	}

	//body, _ := ioutil.ReadAll(response.Body)

	response.Body.Close()

	return response.StatusCode == 200, nil
}

// Dial will dial an IP address through a proxy.
func (p *ProxyInfo) Dial(network string, tcpAddr *net.TCPAddr) (net.Conn, error) {
	dialer, err := proxy.SOCKS5(network, p.address, nil, p.dialer)

	if err != nil {
		return nil, err
	}

	conn, err := dialer.Dial(network, tcpAddr.String())

	if err != nil {
		//log.Println("Proxy dial error: ", err)
		return nil, err
	}

	return conn, nil
}
