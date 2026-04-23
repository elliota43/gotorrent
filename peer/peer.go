package peer

import (
	"bytes"
	"fmt"
	"io"
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

type Handshake struct {
	Pstr     string
	Reserved [8]byte
	InfoHash [20]byte
	PeerID   [20]byte
}

func NewHandshake(infoHash, peerID [20]byte) Handshake {
	return Handshake{
		Pstr:     ProtocolString,
		InfoHash: infoHash,
		PeerID:   peerID,
	}
}

func (h Handshake) Serialize() []byte {
	buf := make([]byte, HandshakeLength)
	buf[0] = byte(len(h.Pstr))
	curr := 1

	curr += copy(buf[curr:], h.Pstr)
	curr += copy(buf[curr:], h.Reserved[:])
	curr += copy(buf[curr:], h.InfoHash[:])
	copy(buf[curr:], h.PeerID[:])

	return buf
}

func ReadHandshake(r io.Reader) (*Handshake, error) {
	lengthBuf := make([]byte, 1)
	if _, err := io.ReadFull(r, lengthBuf); err != nil {
		return nil, err
	}

	pstrlen := int(lengthBuf[0])
	if pstrlen == 0 {
		return nil, fmt.Errorf("peer: invalid protocol string length 0")
	}

	rest := make([]byte, 48+pstrlen)
	if _, err := io.ReadFull(r, rest); err != nil {
		return nil, err
	}

	h := &Handshake{
		Pstr: string(rest[:pstrlen]),
	}

	if h.Pstr != ProtocolString {
		return nil, fmt.Errorf("peer: unexpected protocol string %q", h.Pstr)
	}

	curr := pstrlen
	curr += copy(h.Reserved[:], rest[curr:curr+8])
	curr += copy(h.InfoHash[:], rest[curr:curr+20])
	copy(h.PeerID[:], rest[curr:curr+20])

	return h, nil
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
