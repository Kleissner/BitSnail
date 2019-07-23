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
* DDoS multiple clients at once

This tool was developed just with 1 day of work and is not optimized. The result could be probably far better by tweaking the amount of connections, timeouts and an automated increase/decrease of connection attempts. BitSnail does not detect if the source IP was blocked.

Disclaimer: This tool may only be used for security research purposes. Use it only in a controlled environment. Do not use it for unlawful purposes.

## Use

```
BitSnail [IP:Port or input-file] [optional flag tor=Executable of Tor]

Examples:
BitSnail 1.2.3.4:8333
BitSnail 1.2.3.4:8333 tor=tor.exe
BitSnail Targets.txt
```

The target file must have one peer (in IP:Port form) per line.

Example output:

```
------------------------------------------------------------------------------------------------------------------------
Target                  Active Fake Peers    Attempts to Connect    Currently Sleeping    Handshake Errs    Connect Errs
------------------------------------------------------------------------------------------------------------------------
XX.XXX.XXX.XXX:8333                    68                      2                    55                 0               0
XX.XXX.XX.XXX:8333                     67                      3                    55                 0               0
XX.XXX.XXX.XX:8333                     38                     10                    77                44               0
XX.XX.XXX.XX:8333                      48                     10                    67                36               0
XXX.XX.XXX.XXX:8333                    69                      0                    56                 0               0
XXX.XXX.XX.XXX:8333                    69                      0                    56                 0               0
XXX.XXX.XXX.XXX:8333                   12                      7                   106                50               0
```

BitSnail will initially wait until 3 active fake peers are created before proceeding to create more. It will wait 1 second before creating each new fake peer. Therefore, it may take 2 minutes until all the connection slots in the remote peer are exhausted and the remote peer effectively inaccessible.

* Active Fake Peers: This is the count of fake peers injected into the remote Bitcoin peer. The more, the better, because it means that more connection slots are occupied by fake peers instead of real ones.
* Attempts to Connect: The count of current concurrent connection attempts.
* Currently Sleeping: The count of fake peers not yet injected. They are sleeping and awaiting their turn.
* Handshake errors: The count of handshake errors in the last 5 seconds. If high and active fake peers is high, this may indicate an impact at the target.
* Connection errors: The count of connection errors in the last 5 seconds. If high, it could mean that the remote peer banned the source IP.

To use Tor, you must download the expert bundle from the Tor website. The current latest version for Windows 64-bit is: https://dist.torproject.org/torbrowser/8.5.4/tor-win64-0.4.0.5.zip

If you enable Tor and close the application the Tor processes may still run in the background. Either terminate them manually in the Task Manager or via command line (Windows only):

```
taskkill /F /IM tor.exe
```

### How-to check if the attack is successful

You can go to https://bitnodes.earn.com/ and enter the target IP address and click on "Check Node".

Note that is is only a quick check to see whether or not the peer accepts inbound connections at the time. Bitcoin clients are designed to make outgoing connections themselves; so blocking incoming ones of a peer is not completely isolating it.

### Preventing Local TCP Connection Exhaustion

BitSnail opens up many connections to the remote peer. There is the risk of exhausting local available TCP ports.

In Windows there are 2 settings that affect the TCP performance for opening many connections:
1. `MaxUserPort` number, which limits outbound connections
2. `TcpTimedWaitDelay`, which defines how long ports (connections) are remaining in the state `TIME_WAIT`

To set MaxUserPort to a higher number, run these commands:

```
netsh int ipv4 set dynamicport tcp start=1025 num=64511
netsh int ipv6 set dynamicport tcp start=1025 num=64511
```

To improve `TcpTimedWaitDelay`, merge the following REG file. It sets it to 30 seconds.

```
Windows Registry Editor Version 5.00

[HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters]
"TcpTimedWaitDelay"=dword:0000001E
```

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
The default User Agent is set to `/BitSnail:0.2.0/`. You can change it in `main.go`.

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

Even though it's very unlikely to completely isolate a peer using the current version of BitSnail (in parts because of countermeasures), it is powerful enough to at least temporarily block inbound connections and cause disruption.

## Version History

```
2   7/23/2019

Added support for ddosing multiple peers at once. Improved output.

1   7/22/2019

Initial version.
```
