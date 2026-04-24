package peer

import (
	"fmt"
	"net"
	"time"
)

const (
	ProtocolString  = "BitTorrent protocol"
	HandshakeLength = 49 + len(ProtocolString)
)

type Peer struct {
	IP   net.IP
	Port uint16
}

func (p Peer) String() string {
	return net.JoinHostPort(p.IP.String(), fmt.Sprintf("%d", p.Port))
}

func (p Peer) DialTimeout(timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("tcp", p.String(), timeout)
}
