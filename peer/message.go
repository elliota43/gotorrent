package peer

import (
	"encoding/binary"
	"fmt"
	"io"
)

const MaxBlockSize = 16 * 1024

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

func NewRequestMessage(index, begin, length uint32) *Message {
	payload := make([]byte, 12)

	binary.BigEndian.PutUint32(payload[0:4], index)
	binary.BigEndian.PutUint32(payload[4:8], begin)
	binary.BigEndian.PutUint32(payload[8:12], length)

	return &Message{
		ID:      MsgRequest,
		Payload: payload,
	}
}

func ParseHavePayload(payload []byte) (int, error) {
	if len(payload) != 4 {
		return 0, fmt.Errorf("peer: have payload length = %d, want 4", len(payload))
	}

	return int(binary.BigEndian.Uint32(payload)), nil
}

type PieceBlock struct {
	Index int
	Begin int
	Block []byte
}

func ParsePiecePayload(payload []byte) (*PieceBlock, error) {
	if len(payload) < 8 {
		return nil, fmt.Errorf("peer: piece payload length = %d, want at least 8", len(payload))
	}

	return &PieceBlock{
		Index: int(binary.BigEndian.Uint32(payload[0:4])),
		Begin: int(binary.BigEndian.Uint32(payload[4:8])),
		Block: payload[8:],
	}, nil
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
