package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/net/proxy"
)

var torProxies []string
var rotate int

func torGetProxy() string {
	if len(torProxies) == 0 {
		return ""
	}

	proxy := torProxies[rotate%len(torProxies)]
	rotate++

	return proxy
}

// Init creates all the virtual browsers according to the input
// torRestart is in minutes.
func initTorProxies(count int, torBindIP string, torSocketBase int, torExecutable string, torRestart int) {

	torSocketPort := torSocketBase

	for n := 0; n < count; n++ {

		// find next available port
		nextPort := findAvailablePort("127.0.0.1", torSocketPort)
		if nextPort == 0 {
			break
		}
		torSocketPort = nextPort

		// Spin up a new Tor process
		proxyURL := "127.0.0.1:" + strconv.Itoa(torSocketPort)

		if err := torStart(torBindIP, torSocketPort, torExecutable, torRestart); err != nil {
			continue
		}

		torSocketPort++

		torProxies = append(torProxies, proxyURL)
	}

	if len(torProxies) < count {
		fmt.Printf("Tried to start %d Tor proxies, only started %d successfully\n", count, len(torProxies))
	}
}

// torStart starts a new tor process
func torStart(torBindIP string, torSocketPort int, torExecutable string, torRestart int) (err error) {
	// see https://www.torproject.org/docs/tor-manual-dev.html.en for all command line options
	//    tor.exe -SocksPort [Port] -DataDirectory [Tor Data Directory] -ExitRelay 0 -OutboundBindAddress [IP]

	// Each Tor process needs its own data directory. Otherwise it fails with: "[warn] It looks like another Tor process is running with the same data directory.  Waiting 5 seconds to see if it goes away."
	// Use %Temp%\[Port]\ as data directory.
	dataDirectory := path.Join(os.TempDir(), "tor_"+strconv.Itoa(torSocketPort))
	os.Mkdir(dataDirectory, os.ModePerm)

	// use code from bucket launcher
	var cmd *exec.Cmd
	if torBindIP != "" {
		cmd = exec.Command(torExecutable, "-DataDirectory", dataDirectory, "-SocksPort", strconv.Itoa(torSocketPort), "-ExitRelay", "0", "-OutboundBindAddress", torBindIP, "-HTTPTunnelPort", "0")
	} else {
		cmd = exec.Command(torExecutable, "-DataDirectory", dataDirectory, "-SocksPort", strconv.Itoa(torSocketPort), "-ExitRelay", "0", "-HTTPTunnelPort", "0")
		//fmt.Printf("%s %s %s %s %s %s %s %s %s\n", torExecutable, "-DataDirectory", dataDirectory, "-SocksPort", strconv.Itoa(torSocketPort), "-ExitRelay", "0", "-HTTPTunnelPort", "0")
	}
	cmd.Dir = filepath.Dir(torExecutable)
	cmd.Path = filepath.Base(torExecutable)

	if err := cmd.Start(); err != nil {
		fmt.Printf("torStart: cmd.Start of '%s' failed: %v\n", torExecutable, err)
		return err
	}

	fmt.Printf("torStart: Successfully launched '%s' at port %d\n", torExecutable, torSocketPort)

	// start the watch-dog for auto-restart
	go func(cmd *exec.Cmd, torBindIP string, torSocketPort int, torExecutable string, torRestart int) {
		// wait for the process to exit
		if err := cmd.Wait(); err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				// Other error types may be returned for I/O problems.
				fmt.Printf("torStart: cmd.Wait failed: %v\n", err)
				return
			}
		}

		// restart after a short waiting time
		time.Sleep(4 * time.Second)
		torStart(torBindIP, torSocketPort, torExecutable, torRestart)

		return
	}(cmd, torBindIP, torSocketPort, torExecutable, torRestart)

	// start the daemon to automatically kill the Tor process after the given time
	if torRestart > 0 {
		go func(cmd *exec.Cmd, torRestart int) {
			time.Sleep(time.Duration(torRestart) * time.Minute)

			cmd.Process.Kill()
		}(cmd, torRestart)
	}

	// Start the daemon to kill the Tor process when this process exits. This typically only works on Windows when Ctrl + C is used to kill the process.
	// Future: Maybe there is a way to either kill, or takeover old running processes.
	go func(cmd *exec.Cmd) {
		c := make(chan os.Signal, 2)
		signal.Notify(c, os.Interrupt, os.Kill)
		<-c

		cmd.Process.Kill()
		os.Exit(0)
	}(cmd)

	return nil
}

// DialTor will dial an IP address through Tor.
func DialTor(network string, tcpAddr *net.TCPAddr) (net.Conn, error) {
	dialer, err := proxy.SOCKS5("tcp", torGetProxy(), nil, proxy.Direct)

	if err != nil {
		//log.Println("Tor seems to be down...", err)
		return nil, err
	}

	conn, err := dialer.Dial("tcp", tcpAddr.String())

	if err != nil {
		// log.Println("Tor dial error: ", err)
		return nil, err
	}

	return conn, nil
}

func findAvailablePort(host string, basePort int) (port int) {
	for i := 0; i < 200; i++ {
		conn, _ := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(basePort+i)), time.Millisecond*50)
		if conn == nil {
			// No connection possible, assume port is available
			return basePort + i
		}

		//fmt.Printf("Note: Port %d is not available, skipping.\n", basePort+i)
		conn.Close()
	}

	return 0
}
