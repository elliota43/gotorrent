package main

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net"
	"testing"

	"github.com/elliota43/gotorrent/peer"
	"github.com/elliota43/gotorrent/torrent"
)

func TestWaitUntilReadyAndDownloadOnePiece(t *testing.T) {
	pieceData := []byte("one-piece-download-test-payload")
	pieceHash := sha1.Sum(pieceData)

	meta := &torrent.TorrentMeta{
		Info: torrent.InfoDictionary{
			PieceLength: len(pieceData),
			Pieces:      pieceHash[:],
			Name:        "test.bin",
			Length:      int64(len(pieceData)),
		},
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	serverDone := make(chan error, 1)
	go func() {
		defer close(serverDone)
		defer serverConn.Close()

		msg, err := peer.ReadMessage(serverConn)
		if err != nil {
			serverDone <- err
			return
		}

		if msg == nil || msg.ID != peer.MsgInterested {
			serverDone <- fmt.Errorf("expected interested message, got %#v", msg)
			return
		}

		if _, err := serverConn.Write((&peer.Message{
			ID:      peer.MsgBitfield,
			Payload: []byte{0x80},
		}).Serialize()); err != nil {
			serverDone <- err
			return
		}

		if _, err := serverConn.Write((&peer.Message{
			ID: peer.MsgUnchoke,
		}).Serialize()); err != nil {
			serverDone <- err
			return
		}

		msg, err = peer.ReadMessage(serverConn)
		if err != nil {
			serverDone <- err
			return
		}

		if msg == nil || msg.ID != peer.MsgRequest {
			serverDone <- fmt.Errorf("expected request message, got %#v", msg)
			return
		}

		if len(msg.Payload) != 12 {
			serverDone <- fmt.Errorf("request payload length = %d, want 12", len(msg.Payload))
			return
		}

		index := binary.BigEndian.Uint32(msg.Payload[0:4])
		begin := binary.BigEndian.Uint32(msg.Payload[4:8])
		length := binary.BigEndian.Uint32(msg.Payload[8:12])

		if index != 0 {
			serverDone <- fmt.Errorf("request index = %d, want 0", index)
			return
		}
		if begin != 0 {
			serverDone <- fmt.Errorf("request begin = %d, want 0", begin)
			return
		}
		if int(length) != len(pieceData) {
			serverDone <- fmt.Errorf("request length = %d, want %d", length, len(pieceData))
			return
		}

		payload := make([]byte, 8+len(pieceData))
		binary.BigEndian.PutUint32(payload[0:4], index)
		binary.BigEndian.PutUint32(payload[4:8], begin)
		copy(payload[8:], pieceData)

		if _, err := serverConn.Write((&peer.Message{
			ID:      peer.MsgPiece,
			Payload: payload,
		}).Serialize()); err != nil {
			serverDone <- err
			return
		}
	}()

	session := &peer.PeerSession{
		Conn:   clientConn,
		Peer:   peer.Peer{IP: net.IPv4(127, 0, 0, 1), Port: 6881},
		Choked: true,
	}

	if err := session.WriteMessage(peer.NewInterestedMessage()); err != nil {
		t.Fatalf("WriteMessage(interested) error = %v", err)
	}

	if err := waitUntilReady(session, 0); err != nil {
		t.Fatalf("waitUntilReady() error = %v", err)
	}

	got, err := downloadOnePiece(session, meta, 0)
	if err != nil {
		t.Fatalf("downloadOnePiece() error = %v", err)
	}

	if string(got) != string(pieceData) {
		t.Fatalf("downloaded piece = %q, want %q", got, pieceData)
	}

	if err := verifyPiece(meta, 0, got); err != nil {
		t.Fatalf("verifyPiece() error = %v", err)
	}

	if err := <-serverDone; err != nil {
		t.Fatalf("fake peer error = %v", err)
	}
}
