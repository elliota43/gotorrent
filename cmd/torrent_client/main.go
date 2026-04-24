package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/elliota43/gotorrent/peer"
	"github.com/elliota43/gotorrent/torrent"
	"github.com/elliota43/gotorrent/tracker"
)

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

	fmt.Printf("Announce Request: %#v\n", ar)

	peers, err := ar.RequestPeers(meta.Announce)
	if err != nil {
		log.Fatalf("request peers: %v", err)
	}

	fmt.Printf("Received %d peers\n", len(peers))

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
		}

		sessions = append(sessions, session)

		fmt.Printf("[%02d] %s -> handshake ok, peer id = %q\n",
			res.Index,
			res.Peer.String(),
			string(res.ID[:]),
		)
	}

	fmt.Printf("successful handshakes: %d/%d\n", len(sessions), limit)

	var readWG sync.WaitGroup

	for _, session := range sessions {
		readWG.Add(1)

		go func(s *peer.PeerSession) {
			defer readWG.Done()
			readPeerLoop(s)
		}(session)
	}

	readWG.Wait()
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

		fmt.Printf("[%02d] %s <- %s id=%d payload=%d bytes\n",
			s.Index,
			s.Peer.String(),
			msg.Name(),
			msg.ID,
			len(msg.Payload),
		)

		if len(msg.Payload) > 0 {
			fmt.Printf("[%02d] %s payload: %s\n",
				s.Index,
				s.Peer.String(),
				hex.EncodeToString(firstN(msg.Payload, 64)),
			)
		}
	}
}

func firstN(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}

	return b[:n]
}
