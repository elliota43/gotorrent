package peer

import (
	"net"
)

type PeerSession struct {
	Index    int
	Peer     Peer
	Conn     net.Conn
	PeerID   [20]byte
	Bitfield Bitfield

	Choked bool
}

func (s *PeerSession) Close() error {
	return s.Conn.Close()
}

func (s *PeerSession) ReadMessage() (*Message, error) {
	return ReadMessage(s.Conn)
}

func (s *PeerSession) WriteMessage(msg *Message) error {
	_, err := s.Conn.Write(msg.Serialize())
	return err
}
