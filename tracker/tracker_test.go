package tracker

import (
	"net/url"
	"strings"
	"testing"

	"net"

	"github.com/elliota43/gotorrent/bencode"
)

func TestAnnounceRequest_GetURL_BuildsExpectedQuery(t *testing.T) {
	var infoHash [20]byte
	var peerID [20]byte

	copy(infoHash[:], []byte("12345678901234567890"))
	copy(peerID[:], []byte("-GT0001-123456789012"))

	ar := AnnounceRequest{
		InfoHash:   infoHash,
		PeerID:     peerID,
		Port:       6881,
		Uploaded:   0,
		Downloaded: 0,
		Left:       999,
		Compact:    true,
		Event:      "started",
	}

	got, err := ar.GetURL("http://tracker.example.com/announce")
	if err != nil {
		t.Fatalf("GetURL() error = %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}

	if u.Scheme != "http" {
		t.Fatalf("scheme = %q, want %q", u.Scheme, "http")
	}

	if u.Host != "tracker.example.com" {
		t.Fatalf("host = %q, want %q", u.Host, "tracker.example.com")
	}

	if u.Path != "/announce" {
		t.Fatalf("path = %q, want %q", u.Path, "/announce")
	}

	query := u.RawQuery

	assertQueryHasPrefix(t, query, "info_hash=")
	assertQueryHasPrefix(t, query, "peer_id=")
	assertQueryContains(t, query, "port=6881")
	assertQueryContains(t, query, "uploaded=0")
	assertQueryContains(t, query, "downloaded=0")
	assertQueryContains(t, query, "left=999")
	assertQueryContains(t, query, "compact=1")
	assertQueryContains(t, query, "event=started")
}

func TestAnnounceRequest_GetURL_CompactFalse(t *testing.T) {
	var infoHash [20]byte
	var peerID [20]byte

	copy(infoHash[:], []byte("12345678901234567890"))
	copy(peerID[:], []byte("-GT0001-123456789012"))

	ar := AnnounceRequest{
		InfoHash:   infoHash,
		PeerID:     peerID,
		Port:       6881,
		Uploaded:   10,
		Downloaded: 20,
		Left:       30,
		Compact:    false,
	}

	got, err := ar.GetURL("http://tracker.example.com/announce")
	if err != nil {
		t.Fatalf("GetURL() error = %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}

	query := u.RawQuery
	assertQueryContains(t, query, "compact=0")
	assertQueryContains(t, query, "uploaded=10")
	assertQueryContains(t, query, "downloaded=20")
	assertQueryContains(t, query, "left=30")
}

func TestAnnounceRequest_GetURL_OmitsEmptyEvent(t *testing.T) {
	var infoHash [20]byte
	var peerID [20]byte

	copy(infoHash[:], []byte("12345678901234567890"))
	copy(peerID[:], []byte("-GT0001-123456789012"))

	ar := AnnounceRequest{
		InfoHash: infoHash,
		PeerID:   peerID,
		Port:     6881,
		Left:     100,
		Compact:  true,
		Event:    "",
	}

	got, err := ar.GetURL("http://tracker.example.com/announce")
	if err != nil {
		t.Fatalf("GetURL() error = %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}

	if strings.Contains(u.RawQuery, "event=") {
		t.Fatalf("query unexpectedly contains event: %q", u.RawQuery)
	}
}

func TestAnnounceRequest_GetURL_PreservesExistingQuery(t *testing.T) {
	var infoHash [20]byte
	var peerID [20]byte

	copy(infoHash[:], []byte("12345678901234567890"))
	copy(peerID[:], []byte("-GT0001-123456789012"))

	ar := AnnounceRequest{
		InfoHash: infoHash,
		PeerID:   peerID,
		Port:     6881,
		Left:     100,
		Compact:  true,
	}

	got, err := ar.GetURL("http://tracker.example.com/announce?foo=bar")
	if err != nil {
		t.Fatalf("GetURL() error = %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}

	query := u.RawQuery
	assertQueryContains(t, query, "foo=bar")
	assertQueryContains(t, query, "port=6881")
	assertQueryContains(t, query, "compact=1")
}

func TestAnnounceRequest_GetURL_InvalidBaseURL(t *testing.T) {
	var infoHash [20]byte
	var peerID [20]byte

	ar := AnnounceRequest{
		InfoHash: infoHash,
		PeerID:   peerID,
		Port:     6881,
		Compact:  true,
	}

	_, err := ar.GetURL(":// bad url")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestEscapeBytes_PercentEncodesReservedBytes(t *testing.T) {
	in := []byte{
		0x00,
		0x7f,
		'/',
		'?',
		'&',
		'A',
		'z',
		'0',
		'-',
		'_',
		'.',
		'~',
	}

	got := escapeBytes(in)

	wantParts := []string{
		"%00",
		"%7F",
		"%2F",
		"%3F",
		"%26",
		"A",
		"z",
		"0",
		"-",
		"_",
		".",
		"~",
	}

	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("escapeBytes(%v) = %q, want it to contain %q", in, got, part)
		}
	}
}

func assertQueryContains(t *testing.T, query, want string) {
	t.Helper()
	if !strings.Contains(query, want) {
		t.Fatalf("query = %q, want substring %q", query, want)
	}
}

func assertQueryHasPrefix(t *testing.T, query, key string) {
	t.Helper()

	if !strings.Contains(query, key) {
		t.Fatalf("query = %q, want key %q present", query, key)
	}
}

func TestParseCompactPeers(t *testing.T) {
	input := []byte{
		1, 2, 3, 4, 0x1A, 0xE1, // 1.2.3.4:6881
		5, 6, 7, 8, 0xC8, 0xD5, // 5.6.7.8:51413
	}

	peers, err := parseCompactPeers(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(peers) != 2 {
		t.Fatalf("got %d peers, want 2", len(peers))
	}

	if !peers[0].IP.Equal(net.IPv4(1, 2, 3, 4)) {
		t.Fatalf("peer 0 IP = %v", peers[0].IP)
	}

	if peers[0].Port != 6881 {
		t.Fatalf("peer 0 port = %d, want 6881", peers[0].Port)
	}

	if !peers[1].IP.Equal(net.IPv4(5, 6, 7, 8)) {
		t.Fatalf("peer 1 IP = %v", peers[1].IP)
	}

	if peers[1].Port != 51413 {
		t.Fatalf("peer 1 port = %d, want 51413", peers[1].Port)
	}
}

func TestParseCompactPeers_BadLength(t *testing.T) {
	input := []byte{1, 2, 3, 4, 5}

	_, err := parseCompactPeers(input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParsePeers_Compact(t *testing.T) {
	input := []byte{
		1, 2, 3, 4, 0x1A, 0xE1, // 1.2.3.4:6881
		5, 6, 7, 8, 0xC8, 0xD5, // 5.6.7.8:51413
	}

	peers, err := parsePeers(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(peers) != 2 {
		t.Fatalf("got %d peers, want 2", len(peers))
	}
}

func TestParsePeers_DictionaryList(t *testing.T) {
	input := bencode.List{
		bencode.Dict{
			"ip":      []byte("1.2.3.4"),
			"port":    int64(6881),
			"peer id": []byte("peer-1"),
		},
		bencode.Dict{
			"ip":   []byte("2001:db8::1"),
			"port": int64(51413),
		},
	}

	peers, err := parsePeers(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(peers) != 2 {
		t.Fatalf("got %d peers, want 2", len(peers))
	}

	if !peers[0].IP.Equal(net.ParseIP("1.2.3.4")) {
		t.Fatalf("peer 0 IP = %v", peers[0].IP)
	}

	if peers[0].Port != 6881 {
		t.Fatalf("peer 0 port = %d, want 6881", peers[0].Port)
	}

	if !peers[1].IP.Equal(net.ParseIP("2001:db8::1")) {
		t.Fatalf("peer 1 IP = %v", peers[1].IP)
	}

	if peers[1].Port != 51413 {
		t.Fatalf("peer 1 port = %d, want 51413", peers[1].Port)
	}
}

func TestTrackerResponse_UnmarshalNonCompactPeers(t *testing.T) {
	input := "d8:intervali1800e5:peersld2:ip7:1.2.3.44:porti6881eed2:ip11:2001:db8::14:porti51413eeee"

	var resp Response
	if err := bencode.Unmarshal(strings.NewReader(input), &resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	peers, err := parsePeers(resp.Peers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(peers) != 2 {
		t.Fatalf("got %d peers, want 2", len(peers))
	}
}
