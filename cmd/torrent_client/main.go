package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/elliota43/gotorrent/peer"
	"github.com/elliota43/gotorrent/torrent"
	"github.com/elliota43/gotorrent/tracker"
)

const blockSize = 16 * 1024

type handshakeResult struct {
	Index int
	Peer  peer.Peer
	Conn  net.Conn
	ID    [20]byte
	Err   error
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s <file.torrent>", os.Args[0])
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	meta, err := torrent.Open(f)
	if err != nil {
		log.Fatal(err)
	}

	printTorrentMeta(meta)

	ar := tracker.NewAnnounceRequest(meta, tracker.NewPeerID(), tracker.NewPort())
	ar.Event = "started"

	fmt.Printf("Announce Request: %#v\n", ar)

	peers, announceURL, err := requestPeersFromTrackers(ar, meta)
	if err != nil {
		log.Fatalf("request peers: %v", err)
	}

	fmt.Printf("Received %d peers from %s\n", len(peers), announceURL)

	limit := min(25, len(peers))
	results := make(chan handshakeResult, limit)

	var wg sync.WaitGroup
	for i, p := range peers[:limit] {
		wg.Add(1)

		go func(i int, p peer.Peer) {
			defer wg.Done()

			conn, err := p.DialTimeout(time.Second * 5)
			if err != nil {
				results <- handshakeResult{
					Index: i,
					Peer:  p,
					Err:   fmt.Errorf("dial failed: %w", err),
				}
				return
			}

			_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
			hs, err := peer.CompleteHandshake(conn, meta.InfoHash, ar.PeerID)
			if err != nil {
				conn.Close()
				results <- handshakeResult{
					Index: i,
					Peer:  p,
					Err:   fmt.Errorf("handshake failed: %w", err),
				}
				return
			}

			_ = conn.SetDeadline(time.Time{})

			_, err = conn.Write(peer.NewInterestedMessage().Serialize())
			if err != nil {
				conn.Close()
				results <- handshakeResult{
					Index: i,
					Peer:  p,
					Err:   fmt.Errorf("send interested failed: %w", err),
				}
				return
			}

			results <- handshakeResult{
				Index: i,
				Peer:  p,
				Conn:  conn,
				ID:    hs.PeerID,
				Err:   nil,
			}
		}(i, p)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var sessions []*peer.PeerSession
	for res := range results {
		if res.Err != nil {
			fmt.Printf("[%02d] %s -> %v\n", res.Index, res.Peer.String(), res.Err)
			continue
		}

		session := &peer.PeerSession{
			Index:  res.Index,
			Peer:   res.Peer,
			Conn:   res.Conn,
			PeerID: res.ID,
			Choked: true,
		}

		sessions = append(sessions, session)

		fmt.Printf("[%02d] %s -> handshake ok, peer id = %q\n",
			res.Index,
			res.Peer.String(),
			string(res.ID[:]),
		)
	}

	defer closeSessions(sessions)

	fmt.Printf("successful handshakes: %d/%d\n", len(sessions), limit)

	if len(sessions) == 0 {
		log.Fatal("no successful peer handshakes")
	}

	pieceIndex := 0

	for _, s := range sessions {
		fmt.Printf("[%02d] trying %s for piece %d\n", s.Index, s.Peer.String(), pieceIndex)

		err := waitUntilReady(s, pieceIndex)
		if err != nil {
			fmt.Printf("[%02d] %s -> not usable: %v\n", s.Index, s.Peer.String(), err)
			continue
		}

		pieceData, err := downloadOnePiece(s, meta, pieceIndex)
		if err != nil {
			fmt.Printf("[%02d] %s -> download failed: %v\n", s.Index, s.Peer.String(), err)
			continue
		}

		if err := verifyPiece(meta, pieceIndex, pieceData); err != nil {
			fmt.Printf("[%02d] %s -> bad piece: %v\n", s.Index, s.Peer.String(), err)
			continue
		}

		outPath := fmt.Sprintf("piece-%d.bin", pieceIndex)

		if err := os.WriteFile(outPath, pieceData, 0644); err != nil {
			log.Fatalf("write %s: %v", outPath, err)
		}

		fmt.Printf("downloaded and verified piece %d from %s\n", pieceIndex, s.Peer.String())
		fmt.Printf("wrote %s (%d bytes)\n", outPath, len(pieceData))
		return
	}

	log.Fatalf("could not download piece %d from any peer", pieceIndex)
}

func waitUntilReady(s *peer.PeerSession, pieceIndex int) error {
	s.Choked = true

	_ = s.Conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	defer s.Conn.SetReadDeadline(time.Time{})

	for {
		if !s.Choked && s.Bitfield.HasPiece(pieceIndex) {
			return nil
		}

		msg, err := s.ReadMessage()
		if err != nil {
			return err
		}

		if msg == nil {
			fmt.Printf("[%02d] %s <-  keep-alive while waiting\n", s.Index, s.Peer.String())
			continue
		}

		switch msg.ID {
		case peer.MsgChoke:
			s.Choked = true
			fmt.Printf("[%02d] %s <- choke\n", s.Index, s.Peer.String())

		case peer.MsgUnchoke:
			s.Choked = false
			fmt.Printf("[%02d] %s <- unchoke\n", s.Index, s.Peer.String())

		case peer.MsgBitfield:
			s.Bitfield = peer.Bitfield(append([]byte(nil), msg.Payload...))

			fmt.Printf("[%02d] %s <- bitfield payload=%d bytes hasPiece[%d]=%v\n",
				s.Index,
				s.Peer.String(),
				len(msg.Payload),
				pieceIndex,
				s.Bitfield.HasPiece(pieceIndex),
			)

		case peer.MsgHave:
			haveIndex, err := peer.ParseHavePayload(msg.Payload)
			if err != nil {
				fmt.Printf("[%02d] %s <- invalid have: %v\n", s.Index, s.Peer.String(), err)
				continue
			}

			s.Bitfield.SetPiece(haveIndex)

			fmt.Printf("[%02d] %s <- have piece=%d\n",
				s.Index,
				s.Peer.String(),
				haveIndex,
			)

		default:
			fmt.Printf("[%02d] %s <- %s while waiting payload=%d bytes\n",
				s.Index,
				s.Peer.String(),
				msg.Name(),
				len(msg.Payload),
			)
		}
	}
}

func printTorrentMeta(meta *torrent.TorrentMeta) {
	fmt.Println("=== Torrent Metadata ===")
	fmt.Printf("Announce: %s\n", meta.Announce)

	if len(meta.AnnounceList) > 0 {
		fmt.Println("Announce List:")
		for i, tier := range meta.AnnounceList {
			fmt.Printf("  Tier %d:\n", i+1)
			for _, tracker := range tier {
				fmt.Printf("    - %s\n", tracker)
			}
		}
	}

	if meta.Comment != "" {
		fmt.Printf("Comment: %s\n", meta.Comment)
	}
	if meta.CreatedBy != "" {
		fmt.Printf("Created By: %s\n", meta.CreatedBy)
	}
	if meta.CreationDate != 0 {
		fmt.Printf("Creation Date: %d\n", meta.CreationDate)
	}
	if meta.Encoding != "" {
		fmt.Printf("Encoding: %s\n", meta.Encoding)
	}

	fmt.Println()
	fmt.Println("=== Info Dictionary ===")
	fmt.Printf("Name: %s\n", meta.Info.Name)
	fmt.Printf("Piece Length: %d\n", meta.Info.PieceLength)
	fmt.Printf("Piece Count: %d\n", meta.Info.PieceCount())
	fmt.Printf("Total Length: %d bytes\n", meta.Info.TotalLength())
	fmt.Printf("Last Piece Length: %d bytes\n", meta.Info.LastPieceLength())

	fmt.Printf("Private: %v\n", meta.Info.Private == 1)

	fmt.Printf("Info Hash: %x\n", meta.InfoHash)
	fmt.Printf("Info Bytes Length: %d\n", len(meta.InfoBytes))

	if len(meta.Info.Pieces) > 0 {
		fmt.Printf("Raw Pieces Field Length: %d bytes\n", len(meta.Info.Pieces))
	}

	hashes, err := meta.Info.PieceHashes()
	if err != nil {
		fmt.Printf("Piece Hash Parse Error: %v\n", err)
	} else if len(hashes) > 0 {
		fmt.Printf("First Piece Hash: %s\n", hex.EncodeToString(hashes[0][:]))
	}

	fmt.Println()

	if meta.Info.IsSingleFile() {
		fmt.Println("=== Mode: Single File ===")
		fmt.Printf("Length: %d bytes\n", meta.Info.Length)
		return
	}

	fmt.Println("=== Mode: Multi File ===")
	fmt.Printf("Files: %d\n", len(meta.Info.Files))
	for i, f := range meta.Info.Files {
		fmt.Printf("  [%d] %s (%d bytes)\n", i, strings.Join(f.Path, "/"), f.Length)
	}
}

func requestPeersFromTrackers(ar tracker.AnnounceRequest, meta *torrent.TorrentMeta) ([]peer.Peer, string, error) {
	announceURLs := collectAnnounceURLs(meta)
	if len(announceURLs) == 0 {
		return nil, "", fmt.Errorf("torrent has no announce URLs")
	}

	var failures []string

	for _, announceURL := range announceURLs {
		parsed, err := url.Parse(announceURL)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: invalid announce URL: %v", announceURL, err))
			continue
		}

		switch strings.ToLower(parsed.Scheme) {
		case "http", "https":
		default:
			failures = append(failures, fmt.Sprintf("%s: unsupported tracker scheme %q", announceURL, parsed.Scheme))
			continue
		}

		peers, err := ar.RequestPeers(announceURL)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", announceURL, err))
			continue
		}

		if len(peers) == 0 {
			failures = append(failures, fmt.Sprintf("%s: tracker returned no peers", announceURL))
			continue
		}

		return dedupePeers(peers), announceURL, nil
	}

	return nil, "", fmt.Errorf("all trackers failed: %s", strings.Join(failures, "; "))
}

func collectAnnounceURLs(meta *torrent.TorrentMeta) []string {
	seen := make(map[string]struct{})
	var announceURLs []string

	add := func(announceURL string) {
		announceURL = strings.TrimSpace(announceURL)
		if announceURL == "" {
			return
		}
		if _, ok := seen[announceURL]; ok {
			return
		}
		seen[announceURL] = struct{}{}
		announceURLs = append(announceURLs, announceURL)
	}

	add(meta.Announce)

	for _, tier := range meta.AnnounceList {
		for _, announceURL := range tier {
			add(announceURL)
		}
	}

	return announceURLs
}

func dedupePeers(peers []peer.Peer) []peer.Peer {
	seen := make(map[string]struct{}, len(peers))
	deduped := make([]peer.Peer, 0, len(peers))

	for _, p := range peers {
		key := p.String()
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		deduped = append(deduped, p)
	}

	return deduped
}

func downloadOnePiece(s *peer.PeerSession, meta *torrent.TorrentMeta, pieceIndex int) ([]byte, error) {
	if s.Choked {
		return nil, fmt.Errorf("peer is still choking us")
	}

	if !s.Bitfield.HasPiece(pieceIndex) {
		return nil, fmt.Errorf("peer does not have piece %d", pieceIndex)
	}

	pieceLength64, err := meta.Info.PieceLengthAt(pieceIndex)
	if err != nil {
		return nil, err
	}

	pieceLength := int(pieceLength64)
	pieceBuf := make([]byte, pieceLength)

	requests := make(map[int]int)

	for begin := 0; begin < pieceLength; begin += blockSize {
		length := blockSize
		if begin+length > pieceLength {
			length = pieceLength - begin
		}

		requests[begin] = length

		msg := peer.NewRequestMessage(uint32(pieceIndex), uint32(begin), uint32(length))

		_ = s.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

		_, err = s.Conn.Write(msg.Serialize())
		if err != nil {
			return nil, fmt.Errorf("send request piece=%d begin=%d length=%d: %w",
				pieceIndex,
				begin,
				length,
				err,
			)
		}

		fmt.Printf("[%02d] %s -> request piece=%d begin=%d length=%d\n",
			s.Index,
			s.Peer.String(),
			pieceIndex,
			begin,
			length,
		)
	}

	_ = s.Conn.SetWriteDeadline(time.Time{})

	received := make(map[int]bool)
	receivedBytes := 0

	_ = s.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	defer s.Conn.SetReadDeadline(time.Time{})

	for receivedBytes < pieceLength {
		msg, err := s.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("read while downloading piece: %w", err)
		}

		if msg == nil {
			fmt.Printf("[%02d] %s <- keep-alive while downloading\n", s.Index, s.Peer.String())
			continue
		}

		switch msg.ID {
		case peer.MsgChoke:
			s.Choked = true
			return nil, fmt.Errorf("peer choked us while downloading")
		case peer.MsgUnchoke:
			s.Choked = false
			continue

		case peer.MsgBitfield:
			s.Bitfield = peer.Bitfield(append([]byte(nil), msg.Payload...))
			continue

		case peer.MsgHave:
			haveIndex, err := peer.ParseHavePayload(msg.Payload)
			if err == nil {
				s.Bitfield.SetPiece(haveIndex)
			}
			continue

		case peer.MsgPiece:
			block, err := peer.ParsePiecePayload(msg.Payload)
			if err != nil {
				return nil, err
			}

			if block.Index != pieceIndex {
				fmt.Printf("[%02d] ignoring block for piece=%d while downloading piece=%d\n",
					s.Index,
					block.Index,
					pieceIndex,
				)
				continue
			}

			expectedLength, ok := requests[block.Begin]
			if !ok {
				return nil, fmt.Errorf("unexpected block begin=%d", block.Begin)
			}

			if len(block.Block) != expectedLength {
				return nil, fmt.Errorf("block begin=%d length=%d, want=%d",
					block.Begin,
					len(block.Block),
					expectedLength,
				)
			}

			if block.Begin < 0 || block.Begin+len(block.Block) > len(pieceBuf) {
				return nil, fmt.Errorf("invalid block bounds begin=%d length=%d pieceLength=%d",
					block.Begin,
					len(block.Block),
					len(pieceBuf),
				)
			}

			if received[block.Begin] {
				fmt.Printf("[%02d] duplicate block begin=%d ignored\n", s.Index, block.Begin)
				continue
			}

			copy(pieceBuf[block.Begin:block.Begin+len(block.Block)], block.Block)

			received[block.Begin] = true
			receivedBytes += len(block.Block)

			fmt.Printf("[%02d] %s <- block piece=%d begin=%d length=%d received=%d/%d\n",
				s.Index,
				s.Peer.String(),
				block.Index,
				block.Begin,
				len(block.Block),
				receivedBytes,
				pieceLength,
			)

		default:
			fmt.Printf("[%02d] %s <- %s while downloading payload=%d bytes\n",
				s.Index,
				s.Peer.String(),
				msg.Name(),
				len(msg.Payload),
			)

		}
	}

	return pieceBuf, nil
}

func verifyPiece(meta *torrent.TorrentMeta, pieceIndex int, data []byte) error {
	want, err := meta.Info.PieceHash(pieceIndex)
	if err != nil {
		return err
	}

	got := sha1.Sum(data)

	if got != want {
		return fmt.Errorf("piece %d hash mismatch: got %x want %x", pieceIndex, got, want)
	}
	return nil
}

func readPeerLoop(s *peer.PeerSession) {
	defer s.Close()

	fmt.Printf("[%02d] %s -> starting read loop\n", s.Index, s.Peer.String())

	for {
		msg, err := s.ReadMessage()
		if err != nil {
			fmt.Printf("[%02d] %s -> read error: %v\n", s.Index, s.Peer.String(), err)
			return
		}

		if msg == nil {
			fmt.Printf("[%02d] %s <- keep-alive\n", s.Index, s.Peer.String())
			continue
		}

		switch msg.ID {
		case peer.MsgChoke:
			s.Choked = true
			fmt.Printf("[%02d] %s <- choke\n", s.Index, s.Peer.String())

		case peer.MsgUnchoke:
			s.Choked = false
			fmt.Printf("[%02d] %s <- unchoke\n", s.Index, s.Peer.String())

		case peer.MsgBitfield:
			s.Bitfield = peer.Bitfield(append([]byte(nil), msg.Payload...))
			fmt.Printf("[%02d] %s <- bitfield payload=%d bytes\n",
				s.Index, s.Peer.String(), len(msg.Payload))

		case peer.MsgHave:
			pieceIndex, err := peer.ParseHavePayload(msg.Payload)
			if err != nil {
				fmt.Printf("[%02d] %s <- invalid have: %v\n", s.Index, s.Peer.String(), err)
				continue
			}

			s.Bitfield.SetPiece(pieceIndex)

			fmt.Printf("[%02d] %s <- have piece=%d\n",
				s.Index, s.Peer.String(), pieceIndex)

		case peer.MsgPiece:
			block, err := peer.ParsePiecePayload(msg.Payload)
			if err != nil {
				fmt.Printf("[%02d]  %s <- invalid piece: %v\n", s.Index, s.Peer.String(), err)
				continue
			}

			fmt.Printf("[%02d] %s <- piece index=%d begin=%d block=%d bytes\n",
				s.Index,
				s.Peer.String(),
				block.Index,
				block.Begin,
				len(block.Block),
			)

		default:
			fmt.Printf("[%02d] %s <- %s id=%d payload=%d bytes\n",
				s.Index,
				s.Peer.String(),
				msg.Name(),
				msg.ID,
				len(msg.Payload),
			)
		}
	}
}

func closeSessions(sessions []*peer.PeerSession) {
	for _, s := range sessions {
		if s != nil {
			_ = s.Close()
		}
	}
}
