package tracker

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

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

func NewPeerID() [20]byte {
	var id [20]byte
	copy(id[:], PeerID)
	return id
}

func NewPort() uint16 {
	return Port
}

func (ar AnnounceRequest) GetURL(announceURL string) (string, error) {
	base, err := url.Parse(announceURL)
	if err != nil {
		return "", err
	}

	q := buildTrackerQuery(ar)

	if base.RawQuery == "" {
		base.RawQuery = q
	} else {
		base.RawQuery = base.RawQuery + "&" + q
	}
	return base.String(), nil
}

func buildTrackerQuery(ar AnnounceRequest) string {
	params := []string{
		"info_hash=" + escapeBytes(ar.InfoHash[:]),
		"peer_id=" + escapeBytes(ar.PeerID[:]),
		"port=" + strconv.FormatUint(uint64(ar.Port), 10),
		"uploaded=" + strconv.FormatInt(ar.Uploaded, 10),
		"downloaded=" + strconv.FormatInt(ar.Downloaded, 10),
		"left=" + strconv.FormatInt(ar.Left, 10),
	}

	if ar.Compact {
		params = append(params, "compact=1")
	} else {
		params = append(params, "compact=0")
	}

	if ar.Event != "" {
		params = append(params, "event="+url.QueryEscape(ar.Event))
	}

	return strings.Join(params, "&")
}

func escapeBytes(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		if (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '.' || c == '-' || c == '_' || c == '~' {
			sb.WriteByte(c)
		} else {
			sb.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	return sb.String()
}
