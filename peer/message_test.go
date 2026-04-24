package peer

import (
	"bytes"
	"testing"
)

func TestMessageSerializeKeepAlive(t *testing.T) {
	var msg *Message

	got := msg.Serialize()
	want := []byte{0x00, 0x00, 0x00, 0x00}

	if !bytes.Equal(got, want) {
		t.Fatalf("Serialize() = %x, want %x", got, want)
	}
}

func TestMessageSerializeInterested(t *testing.T) {
	msg := &Message{
		ID: MsgInterested,
	}

	got := msg.Serialize()
	want := []byte{
		0x00, 0x00, 0x00, 0x01,
		0x02,
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("Serialize() = %x, want %x", got, want)
	}
}

func TestMessageSerializeHave(t *testing.T) {
	msg := &Message{
		ID:      MsgHave,
		Payload: []byte{0x00, 0x00, 0x00, 0x05},
	}

	got := msg.Serialize()
	want := []byte{
		0x00, 0x00, 0x00, 0x05,
		0x04,
		0x00, 0x00, 0x00, 0x05,
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("Serialize() = %x, want %x", got, want)
	}
}

func TestReadMessageKeepAlive(t *testing.T) {
	r := bytes.NewReader([]byte{
		0x00, 0x00, 0x00, 0x00,
	})

	msg, err := ReadMessage(r)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}

	if msg != nil {
		t.Fatalf("ReadMessage() = %#v, want nil keep-alive", msg)
	}
}

func TestReadMessageInterested(t *testing.T) {
	r := bytes.NewReader([]byte{
		0x00, 0x00, 0x00, 0x01,
		0x02,
	})

	msg, err := ReadMessage(r)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}

	if msg == nil {
		t.Fatal("ReadMessage() = nil, want interested message")
	}

	if msg.ID != MsgInterested {
		t.Fatalf("msg.ID = %v, want %v", msg.ID, MsgInterested)
	}

	if len(msg.Payload) != 0 {
		t.Fatalf("len(msg.Payload) = %d, want 0", len(msg.Payload))
	}
}

func TestReadMessageBitfield(t *testing.T) {
	r := bytes.NewReader([]byte{
		0x00, 0x00, 0x00, 0x03,
		0x05,
		0b10100000,
		0b00000001,
	})

	msg, err := ReadMessage(r)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}

	if msg == nil {
		t.Fatal("ReadMessage() = nil, want bitfield message")
	}

	if msg.ID != MsgBitfield {
		t.Fatalf("msg.ID = %v, want %v", msg.ID, MsgBitfield)
	}

	wantPayload := []byte{0b10100000, 0b00000001}
	if !bytes.Equal(msg.Payload, wantPayload) {
		t.Fatalf("msg.Payload = %08b, want %08b", msg.Payload, wantPayload)
	}
}

func TestReadMessageShortLengthPrefix(t *testing.T) {
	r := bytes.NewReader([]byte{
		0x00, 0x00,
	})

	_, err := ReadMessage(r)
	if err == nil {
		t.Fatal("ReadMessage() error = nil, want error")
	}
}

func TestReadMessageShortPayload(t *testing.T) {
	r := bytes.NewReader([]byte{
		0x00, 0x00, 0x00, 0x05,
		0x04,
		0x00, 0x00,
	})

	_, err := ReadMessage(r)
	if err == nil {
		t.Fatal("ReadMessage() error = nil, want error")
	}
}

func TestMessageIDString(t *testing.T) {
	tests := []struct {
		id   MessageID
		want string
	}{
		{MsgChoke, "choke"},
		{MsgUnchoke, "unchoke"},
		{MsgInterested, "interested"},
		{MsgNotInterested, "not interested"},
		{MsgHave, "have"},
		{MsgBitfield, "bitfield"},
		{MsgRequest, "request"},
		{MsgPiece, "piece"},
		{MsgCancel, "cancel"},
		{MessageID(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.id.String()
			if got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
