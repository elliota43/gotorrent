package peer

import (
	"encoding/binary"
	"io"
)

type MessageID uint8

const (
	MsgChoke         MessageID = 0
	MsgUnchoke       MessageID = 1
	MsgInterested    MessageID = 2
	MsgNotInterested MessageID = 3
	MsgHave          MessageID = 4
	MsgBitfield      MessageID = 5
	MsgRequest       MessageID = 6
	MsgPiece         MessageID = 7
	MsgCancel        MessageID = 8
)

type Message struct {
	ID      MessageID
	Payload []byte
}

func (id MessageID) String() string {
	switch id {
	case MsgChoke:
		return "choke"
	case MsgUnchoke:
		return "unchoke"
	case MsgInterested:
		return "interested"
	case MsgNotInterested:
		return "not interested"
	case MsgHave:
		return "have"
	case MsgBitfield:
		return "bitfield"
	case MsgRequest:
		return "request"
	case MsgPiece:
		return "piece"
	case MsgCancel:
		return "cancel"
	default:
		return "unknown"
	}
}

// Serialize serializes a message into a buffer
// <length prefix><message-id><payload>
// 'nil' messages are interpreted as keep-alive messages.
func (m *Message) Serialize() []byte {
	if m == nil {
		return make([]byte, 4)
	}

	length := uint32(len(m.Payload) + 1) // +1 for message id
	buf := make([]byte, 4+length)
	binary.BigEndian.PutUint32(buf[0:4], length)
	buf[4] = byte(m.ID)
	copy(buf[5:], m.Payload)
	return buf
}

func ReadMessage(r io.Reader) (*Message, error) {
	var lengthBuf [4]byte

	if _, err := io.ReadFull(r, lengthBuf[:]); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lengthBuf[:])

	if length == 0 {
		return nil, nil // keep-alive msg
	}

	msgBuf := make([]byte, length)

	if _, err := io.ReadFull(r, msgBuf); err != nil {
		return nil, err
	}

	return &Message{
		ID:      MessageID(msgBuf[0]),
		Payload: msgBuf[1:],
	}, nil
}

func (m *Message) Name() string {
	if m == nil {
		return "keep-alive"
	}

	return m.ID.String()
}

func NewInterestedMessage() *Message {
	return &Message{
		ID: MsgInterested,
	}
}
