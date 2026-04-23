package tracker

import "testing"

func TestNewPeerID(t *testing.T) {
	id := NewPeerID()

	if id != PeerID {
		t.Fatalf("expected %q, got %q", PeerID, id)
	}
}

func TestNewPort(t *testing.T) {
	port := NewPort()

	if port != Port {
		t.Fatalf("expected %d, got %d", Port, port)
	}
}
