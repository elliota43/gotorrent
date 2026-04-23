package peer

import (
	"bytes"
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

func CompleteHandshake(conn net.Conn, infoHash, peerID [20]byte) (*Handshake, error) {
	hs := NewHandshake(infoHash, peerID)

	if _, err := conn.Write(hs.Serialize()); err != nil {
		return nil, err
	}

	resp, err := ReadHandshake(conn)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(resp.InfoHash[:], infoHash[:]) {
		return nil, fmt.Errorf("peer: info hash mismatch")
	}

	return resp, nil
}
