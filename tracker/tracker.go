package tracker

import (
	"net"

	"github.com/elliota43/gotorrent/torrent"
)

type AnnounceRequest struct {
	InfoHash   [20]byte
	PeerID     [20]byte
	Port       uint16
	Uploaded   int64
	Downloaded int64
	Left       int64
	Compact    bool
	Event      string
}

type Response struct {
	Interval int
	Peers    []Peer
}

type Peer struct {
	IP   net.IP
	Port uint16
}

const (
	PeerID = "-GT0001-6f3a9c1d2e4b"
	Port   = 6881
)

func NewAnnounceRequest(meta *torrent.TorrentMeta, peerID [20]byte, port uint16) AnnounceRequest {
	return AnnounceRequest{
		InfoHash:   meta.InfoHash,
		PeerID:     peerID,
		Port:       port,
		Uploaded:   0,
		Downloaded: 0,
		Left:       meta.TotalLength(),
		Compact:    true,
	}
}

func NewPeerID() string {
	return PeerID
}

func NewPort() uint16 {
	return Port
}
