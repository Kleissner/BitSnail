/*
This file is forked from https://github.com/agamble/btc-crawler.
*/
package main

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"syscall"
	"time"

	"github.com/btcsuite/btcd/wire"
)

type Node struct {
	TcpAddr   *net.TCPAddr
	conn      net.Conn
	Adjacents []*Node
	PVer      uint32
	btcNet    wire.BitcoinNet
	Online    bool
	Onion     bool
	Services  uint64
	UserAgent string

	doneC   chan struct{}
	outPath string

	ListenTxs  bool
	ListenBlks bool
}

type StampedInv struct {
	InvVects  []*wire.InvVect
	Timestamp time.Time
}

type StampedSighting struct {
	Timestamp time.Time
	InvVect   *wire.InvVect
}

var onioncatrange = net.IPNet{IP: net.ParseIP("FD87:d87e:eb43::"),
	Mask: net.CIDRMask(48, 128)}

// Checks if a node's IP address falls within the special 'Tor Range'
func (n *Node) IsTorNode() bool {
	// bitcoind encodes a .onion address as a 16 byte number by decoding the
	// address prior to the .onion (i.e. the key hash) base32 into a ten
	// byte number. it then stores the first 6 bytes of the address as
	// 0xfD, 0x87, 0xD8, 0x7e, 0xeb, 0x43
	// this is the same range used by onioncat, part of the
	// RFC4193 Unique local IPv6 range.
	// In summary the format is:
	// { magic 6 bytes, 10 bytes base32 decode of key hash }
	return onioncatrange.Contains(n.TcpAddr.IP)
}

// Returns true if the node is an IPv6 node.
func (n *Node) IsIpv6() bool {
	return n.TcpAddr.IP.To4() == nil
}

// Attempt to form a TCP connection to the node.
func (n *Node) Connect() error {
	if n.IsTorNode() {
		// Onion Address
		conn, err := DialTor("tcp", n.TcpAddr)
		if err != nil {
			// log.Println("Tor connect error: ", err)
			return err
		}
		n.conn = conn
	} else {
		conn, err := net.DialTimeout("tcp", n.TcpAddr.String(), 30*time.Second)
		if err != nil {
			return err
		}
		n.conn = conn
	}

	return nil
}

// Handshake performs the handshake operation according to the Bitcoin protocol.
func (n *Node) Handshake() error {
	nonce, err := wire.RandomUint64()

	if err != nil {
		log.Print("Generating nonce error:", err)
		return err
	}

	//verMsg, err := wire.NewMsgVersionFromConn(n.conn, nonce, 0)
	meAddr, youAddr := n.conn.LocalAddr(), n.conn.RemoteAddr()
	me := wire.NewNetAddress(meAddr.(*net.TCPAddr), wire.SFNodeNetwork)
	you := wire.NewNetAddress(youAddr.(*net.TCPAddr), wire.SFNodeNetwork)
	verMsg := wire.NewMsgVersion(me, you, nonce, 0)
	verMsg.UserAgent = clientUserAgent

	if err != nil {
		log.Print("Create version message error:", err)
		return err
	}

	n.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	defer n.conn.SetWriteDeadline(time.Time{})
	err = wire.WriteMessage(n.conn, verMsg, n.PVer, n.btcNet)

	if err != nil {
		log.Print("Write version message error:", err)
		return err
	}

	res, err := n.receiveMessageTimeout("version")

	if err != nil {
		return err
	}

	resVer, ok := res.(*wire.MsgVersion)

	if !ok {
		log.Print("Something failed getting version")
	}

	n.PVer = uint32(resVer.ProtocolVersion)
	n.UserAgent = resVer.UserAgent
	n.Services = uint64(resVer.Services)

	n.receiveMessageTimeout("verack")

	return nil
}

func (n *Node) pong(ping *wire.MsgPing) {
	pongMsg := wire.NewMsgPong(ping.Nonce)

	for i := 0; i < 2; i++ {
		n.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		defer n.conn.SetWriteDeadline(time.Time{})

		err := wire.WriteMessage(n.conn, pongMsg, n.PVer, n.btcNet)

		if err != nil {
			log.Println("Failed to send pong", err)
			continue
		}

		return
	}
}

// Receive a message with command string within the commands slice.
func (n *Node) ReceiveMessage(commands []string) (wire.Message, error) {
	for i := 0; i < 50; i++ {
		msg, _, err := wire.ReadMessage(n.conn, n.PVer, n.btcNet)

		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return nil, netErr
			}

			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == syscall.ECONNRESET.Error() {
				return nil, opErr
			}

			// stop trying if there's an IO error
			if err == io.EOF || err == io.ErrUnexpectedEOF || err == io.ErrClosedPipe {
				return nil, err
			}

			// otherwise we've received some generic error, and try again
			continue
		}

		// Always respond to a ping right away
		if ping, ok := msg.(*wire.MsgPing); ok && wire.CmdPing == msg.Command() {
			n.pong(ping)
			continue
		}

		for _, command := range commands {
			if command == msg.Command() {
				return msg, nil
			}
		}
	}

	return nil, errors.New("Failed to receive a message from node")
}

// Setup is Connect and Handshake as one method.
func (n *Node) Setup() error {
	err := n.Connect()

	if err != nil {
		return err
	}

	err = n.Handshake()

	if err != nil {
		return err
	}

	return nil
}

func exists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func (n *Node) createFiles() {
	if !exists(n.txnFilePath()) {
		os.Create(n.txnFilePath())
	}

	if !exists(n.blockFilePath()) {
		os.Create(n.blockFilePath())
	}
}

func (n *Node) txnFilePath() string {
	return n.outPath + "-txn"
}

func (n *Node) blockFilePath() string {
	return n.outPath + "-block"
}

// InvWriter starts the goroutine which manages disk writing for this node.
// Should only be called as its own goroutine.
func (n *Node) InvWriter(dataDirName string, node <-chan *StampedInv) {
	n.outPath = path.Join(dataDirName, n.String())

	n.createFiles()

	txnFile, err := os.OpenFile(n.txnFilePath(), os.O_WRONLY|os.O_APPEND, 0666)
	defer txnFile.Close()

	if err != nil {
		fmt.Printf("Error opening file '%s': %v\n", n.txnFilePath(), err)
		return
		//panic(err)
	}

	blkFile, err := os.OpenFile(n.blockFilePath(), os.O_WRONLY|os.O_APPEND, 0666)
	defer blkFile.Close()

	if err != nil {
		panic(err)
	}

	txnEnc := gob.NewEncoder(txnFile)
	blkEnc := gob.NewEncoder(blkFile)
	stampedSightingHolder := new(StampedSighting)

	for stampedInvs := range node {
		n.WriteInv(txnEnc, blkEnc, stampedSightingHolder, stampedInvs)
	}
}

// WriteInv writes a specific inv message to disk.
func (n *Node) WriteInv(txnEnc *gob.Encoder, blkEnc *gob.Encoder, stampedSightingHolder *StampedSighting, stampedInvs *StampedInv) {
	if stampedSightingHolder == nil {
		stampedSightingHolder = new(StampedSighting)
	}

	stampedSightingHolder.Timestamp = stampedInvs.Timestamp

	for _, invVect := range stampedInvs.InvVects {
		stampedSightingHolder.InvVect = invVect
		var err error
		if n.ListenTxs && invVect.Type == wire.InvTypeTx {
			err = txnEnc.Encode(stampedSightingHolder)
		} else if n.ListenBlks && invVect.Type == wire.InvTypeBlock {
			err = blkEnc.Encode(stampedSightingHolder)
		}

		if err != nil {
			panic(err)
		}
	}
}

func (n *Node) Ping() {
	nonce, _ := wire.RandomUint64()
	n.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	defer n.conn.SetWriteDeadline(time.Time{})
	_ = wire.WriteMessage(n.conn, wire.NewMsgPing(nonce), n.PVer, n.btcNet)
}

// Watch begins listening to the node. Requires an initial connect beforehand.
// Should be run as its own Goroutine.
/*
func (n *Node) Watch(progressC chan<- *watchProgress, stopC chan<- string, addrC chan<- []*wire.NetAddress, dataDirName string) {

	resultC := make(chan *StampedInv, 1)

	invWriterC := make(chan *StampedInv, 1)
	defer close(invWriterC)

	go n.InvWriter(dataDirName, invWriterC)

	if err := n.Setup(); err != nil {
		stopC <- n.String()
		return
	}

	pingTicker := time.NewTicker(time.Minute * 1)
	defer pingTicker.Stop()

	// use a ticker to monitor watcher progress
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	go n.Inv(resultC, addrC)

	countProcessed := 0

	nilCount := 0
	for {
		select {
		case <-pingTicker.C:
			n.Ping()
		case <-n.doneC:
			return
		case <-ticker.C:
			progressC <- &watchProgress{address: n.String(), uniqueInvSeen: countProcessed}
		case stampedInv := <-resultC:
			if stampedInv == nil {
				if nilCount > 5 {
					stopC <- n.String()
					return
				}
				nilCount++
			} else {
				nilCount = 0
			}

			go n.Inv(resultC, addrC)

			if stampedInv != nil {
				invWriterC <- stampedInv
				countProcessed += len(stampedInv.InvVects)
			}
		}
	}
}*/

// Inv receives and accordingly processes a received inv message.
// Send unsolicited addr messages upstream to the dispatcher
func (n *Node) Inv(invC chan<- *StampedInv, addrC chan<- []*wire.NetAddress) {
	res, err := n.ReceiveMessage([]string{"inv", "addr"})
	now := time.Now()

	if err != nil {
		invC <- nil
		return
	}

	switch res := res.(type) {
	case *wire.MsgInv:
		sighting := new(StampedInv)
		sighting.Timestamp = now
		sighting.InvVects = res.InvList[:]
		invC <- sighting
	case *wire.MsgAddr:
		addrC <- res.AddrList
		invC <- nil
	default:
		invC <- nil
	}

}

// StopWatching, called synchronously
func (n *Node) StopWatching() {
	n.doneC <- struct{}{}
	return
}

func (n *Node) receiveMessageTimeout(command string) (wire.Message, error) {
	n.conn.SetReadDeadline(time.Time(time.Now().Add(30 * time.Second)))
	defer n.conn.SetReadDeadline(time.Time{})

	msg, err := n.ReceiveMessage([]string{command})

	if err != nil {
		return nil, err
	}

	return msg, nil
}

// GetAddr asks the node for its konwn addresses
func (n *Node) GetAddr() ([]*wire.NetAddress, error) {
	getAddrMsg := wire.NewMsgGetAddr()
	n.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	defer n.conn.SetWriteDeadline(time.Time{})
	err := wire.WriteMessage(n.conn, getAddrMsg, n.PVer, n.btcNet)

	if err != nil {
		return nil, err
	}

	res, err := n.receiveMessageTimeout("addr")

	if err != nil {
		return nil, err
	}

	if res == nil {
		// return empty adjacents if we receive no response
		return nil, nil
	}

	resAddrMsg := res.(*wire.MsgAddr)

	addrList := resAddrMsg.AddrList

	// allocate the memory in advance!
	n.Adjacents = make([]*Node, len(addrList))

	return addrList, nil
}

// Close the connection with a peer
func (n *Node) Close() error {
	if n.conn == nil {
		return nil
	}

	err := n.conn.Close()
	if err != nil {
		log.Println("Closing connection error:", err)
		return err
	}

	n.conn = nil

	return nil
}

// String gives the string representation of this node, the string of the IP address.
func (n *Node) String() string {
	return n.TcpAddr.String()
}

// IsValid returns whether the node contains a valid address.
func (n *Node) IsValid() bool {

	// obviously a port number of zero won't work
	if n.TcpAddr.Port == 0 {
		return false
	}

	return true
}

// MarshalJSON returns the JSON format of the node.
func (n *Node) MarshalJSON() ([]byte, error) {
	adjsStrs := make([]string, len(n.Adjacents))

	for i, adj := range n.Adjacents {
		adjsStrs[i] = adj.String()
	}

	return json.Marshal(struct {
		Address   string
		Adjacents []string
		PVer      uint32
		Online    bool
		Onion     bool
		Services  uint64
		UserAgent string
	}{
		Address:   n.TcpAddr.String(),
		Adjacents: adjsStrs,
		PVer:      n.PVer,
		Onion:     n.Onion,
		Online:    n.Online,
		Services:  n.Services,
		UserAgent: n.UserAgent,
	})
}

// NewNode creates a new node from an IP address struct
func NewNode(tcpAddr *net.TCPAddr) *Node {
	n := new(Node)
	n.TcpAddr = tcpAddr
	n.btcNet = wire.MainNet
	n.doneC = make(chan struct{}, 1)

	return n
}

// ConnectTor Attempt to form a TCP connection to the node.
func (n *Node) ConnectTor() (net.Conn, error) {

	conn, err := DialTor("tcp", n.TcpAddr)
	if err != nil {
		return nil, err
	}
	n.conn = conn

	return conn, nil
}

// Attempt to form a TCP connection to the node.
func (n *Node) Connect2() (net.Conn, error) {

	conn, err := net.DialTimeout("tcp", n.TcpAddr.String(), 30*time.Second)
	if err != nil {
		return nil, err
	}
	n.conn = conn

	return conn, nil
}
