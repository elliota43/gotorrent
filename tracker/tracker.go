package tracker

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/elliota43/gotorrent/bencode"
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
	Interval      int64  `bencode:"interval"`
	Peers         any    `bencode:"peers"`
	FailureReason string `bencode:"failure reason"`
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

func (ar AnnounceRequest) RequestPeers(announceURL string) ([]Peer, error) {

	fullURL, err := ar.GetURL(announceURL)
	if err != nil {
		return nil, err
	}

	c := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := c.Get(fullURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var trackerResp Response
	if err := bencode.Unmarshal(resp.Body, &trackerResp); err != nil {
		return nil, err
	}

	if trackerResp.FailureReason != "" {
		return nil, fmt.Errorf("tracker: %s", trackerResp.FailureReason)
	}

	peers, err := parsePeers(trackerResp.Peers)
	if err != nil {
		return nil, err
	}

	return peers, nil
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

func parseCompactPeers(b []byte) ([]Peer, error) {
	if len(b)%6 != 0 {
		return nil, fmt.Errorf("tracker: compact peers length %d is not a multiple of 6", len(b))
	}

	peers := make([]Peer, 0, len(b)/6)
	for i := 0; i < len(b); i += 6 {
		ip := net.IPv4(b[i], b[i+1], b[i+2], b[i+3])
		port := uint16(b[i+4])<<8 | uint16(b[i+5])

		peers = append(peers, Peer{
			IP:   ip,
			Port: port,
		})
	}

	return peers, nil
}

func parsePeers(src any) ([]Peer, error) {
	switch peers := src.(type) {
	case []byte:
		return parseCompactPeers(peers)
	case bencode.List:
		return parsePeerList(peers)
	case nil:
		return nil, fmt.Errorf("tracker: missing peers in response")
	default:
		return nil, fmt.Errorf("tracker: unsupported peers value type %T", src)
	}
}

func parsePeerList(list bencode.List) ([]Peer, error) {
	peers := make([]Peer, 0, len(list))
	for i, item := range list {
		dict, ok := item.(bencode.Dict)
		if !ok {
			return nil, fmt.Errorf("tracker: peer list entry %d has type %T, want dictionary", i, item)
		}

		ipValue, ok := dict["ip"]
		if !ok {
			return nil, fmt.Errorf("tracker: peer list entry %d missing ip", i)
		}

		ipBytes, ok := ipValue.([]byte)
		if !ok {
			return nil, fmt.Errorf("tracker: peer list entry %d ip has type %T, want byte string", i, ipValue)
		}

		ip := net.ParseIP(string(ipBytes))
		if ip == nil {
			return nil, fmt.Errorf("tracker: peer list entry %d has invalid ip %q", i, string(ipBytes))
		}

		portValue, ok := dict["port"]
		if !ok {
			return nil, fmt.Errorf("tracker: peer list entry %d missing port", i)
		}

		port, ok := portValue.(int64)
		if !ok {
			return nil, fmt.Errorf("tracker: peer list entry %d port has type %T, want integer", i, portValue)
		}

		if port < 0 || port > 65535 {
			return nil, fmt.Errorf("tracker: peer list entry %d port %d out of range", i, port)
		}

		peers = append(peers, Peer{
			IP:   ip,
			Port: uint16(port),
		})
	}

	return peers, nil
}
