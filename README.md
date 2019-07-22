# BitSnail

BitSnail - an experimental program to exhaust inbound TCP connections of a Bitcoin client (a Bitcoin full node). This could possibly (in a worst case scenario, unlikely at the current point) slow down the entire Bitcoin P2P network if used in a scaled up attack.
This tool is just an initial version developed for further testing. At the current stage, it is unlikely to have a major impact due to countermeasures already applied by the major Bitcoin client (Bitcoin Core).

```
    .----.   ₿   ₿
   / .-"-.`.  \v/
   | | '\ \ \_/ )
 ,-\ `-.' /.'  /
'---`----'----'
```

See https://peterkleissner.com/2019/07/22/security-audit-of-the-bitcoin-p2p-network-getting-started/ for details of the security research behind this. It explains how to get started to audit the Bitcoin P2P network.

Features:
* The first Bitcoin peer ddos software!
* Modify it to to test a variety of attacks.
* Supports Tor as proxy

This tool was developed just with 1 day of work and is not optimized. The result could be probably far better by tweaking the amount of connections, timeouts and an automated increase/decrease of connection attempts. BitSnail does not detect if the source IP was blocked.

Disclaimer: This tool may only be used for security research purposes. Use it only in a controlled environment. Do not use it for unlawful purposes.

## Use

```
BitSnail [IP:Port] [optional flag tor=Executable of Tor]

Examples:
BitSnail 1.2.3.4:8333
BitSnail 1.2.3.4:8333 tor=tor.exe
```

Example output:

```
Wait 10 seconds for 4 Tor proxy instances to connect
Try to create 125 concurrent fake peers, target is X.X.X.X:8333. Initially it will wait for 3 active fake peers and then wait 1 second before creating each new fake peer.
---------
Active Fake Peers 0 -- Attempts to connect 0 -- Currently sleeping 125 -- handshake errs 0
Active Fake Peers 3 -- Attempts to connect 2 -- Currently sleeping 120 -- handshake errs 0
Active Fake Peers 4 -- Attempts to connect 3 -- Currently sleeping 118 -- handshake errs 0
Active Fake Peers 10 -- Attempts to connect 2 -- Currently sleeping 113 -- handshake errs 0
Active Fake Peers 12 -- Attempts to connect 5 -- Currently sleeping 108 -- handshake errs 0
Active Fake Peers 15 -- Attempts to connect 7 -- Currently sleeping 103 -- handshake errs 0
Active Fake Peers 18 -- Attempts to connect 9 -- Currently sleeping 98 -- handshake errs 0
Active Fake Peers 23 -- Attempts to connect 9 -- Currently sleeping 93 -- handshake errs 0
Active Fake Peers 27 -- Attempts to connect 10 -- Currently sleeping 88 -- handshake errs 0
Active Fake Peers 28 -- Attempts to connect 13 -- Currently sleeping 84 -- handshake errs 0
Active Fake Peers 31 -- Attempts to connect 16 -- Currently sleeping 78 -- handshake errs 0
Active Fake Peers 28 -- Attempts to connect 24 -- Currently sleeping 73 -- handshake errs 0
Active Fake Peers 34 -- Attempts to connect 23 -- Currently sleeping 68 -- handshake errs 0
...
```

BitSnail will initially wait until 3 active fake peers are created before proceeding to create more. It will wait 1 second before creating each new fake peer. Therefore, it may take 2 minutes until all the connection slots in the remote peer are exhausted and the remote peer effectively inaccessible.

To use Tor, you must download the expert bundle from the Tor website. The current latest version for Windows 64-bit is: https://dist.torproject.org/torbrowser/8.5.4/tor-win64-0.4.0.5.zip

If you enable Tor and close the application the Tor processes may still run in the background. Either terminate them manually in the Task Manager or via command line (Windows only):

```
taskkill /F /IM tor.exe
```

## How-to check if the attack is successful

You can go to https://bitnodes.earn.com/ and enter the target IP address and click on "Check Node".

Note that is is only a quick check to see whether or not the peer accepts inbound connections at the time. Bitcoin clients are designed to make outgoing connections themselves; so blocking incoming ones of a peer is not completely isolating it.

## Compile

You need [Go](https://golang.org/dl/) to compile the project. Before compilation you need to download the dependencies:

```
go get -u github.com/btcsuite/btcd/wire
```

To compile the project (on Windows this will create the `BitSnail.exe`):

```
go build
```

## Privacy

You can enable Tor as described in the use section to conceal your real IP address. This can be useful to bypass IP blocking.
The default User Agent is set to `/BitSnail:0.1.0/`. You can change it in `main.go`.

## Countermeasures

There are a couple of countermeasures that can be applied by Bitcoin clients. The most obvious ones are:
* Limit incoming connections (and peers) per IP addresses
* When the connection pool is full, evict connections according to certain characteristics (as done by the Bitcoin Core client)

The [Bitcoin Core client](https://github.com/bitcoin/bitcoin) already has a bunch of countermeasures that make it really difficult
to completely isolate a peer. In the function `AttemptToEvictConnection` it will try evict connections when the connection pool is full. Here is the comment of that function:

> Try to find a connection to evict when the node is full.
> Extreme care must be taken to avoid opening the node to attacker triggered network partitioning.
> The strategy used here is to protect a small number of peers for each of several distinct characteristics which are difficult to forge.
> In order to partition a node the attacker must be simultaneously better at all of them than honest peers.


## Version History

```
1   7/22/2019

Initial version.
```
