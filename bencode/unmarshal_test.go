package bencode

import (
	"strings"
	"testing"
)

func TestUnmarshalStruct(t *testing.T) {
	type Info struct {
		Name        string `bencode:"name"`
		PieceLength int    `bencode:"piece length"`
	}

	type Torrent struct {
		Announce string `bencode:"announce"`
		Info     Info   `bencoe:"info"`
	}

	input := "d8:announce14:http://tracker4:infod4:name8:test.iso12:piece lengthi16384eee"

	var got Torrent
	err := Unmarshal(strings.NewReader(input), &got)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Announce != "http://tracker" {
		t.Fatalf("got announce %q, want %q", got.Announce, "http://tracker")
	}
	if got.Info.Name != "test.iso" {
		t.Fatalf("got info.name %q, want %q", got.Info.Name, "test.iso")
	}
	if got.Info.PieceLength != 16384 {
		t.Fatalf("got piece length %d, want %d", got.Info.PieceLength, 16384)
	}
}

func TestUnmarshalStringField(t *testing.T) {
	type S struct {
		Name string `bencode:"name"`
	}

	input := "d4:name4:spame"

	var got S
	err := Unmarshal(strings.NewReader(input), &got)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Name != "spam" {
		t.Fatalf("got %q, want %q", got.Name, "spam")
	}
}

func TestUnmarshalBytesField(t *testing.T) {
	type S struct {
		Pieces []byte `bencode:"pieces"`
	}

	input := "d6:pieces4:spame"

	var got S
	err := Unmarshal(strings.NewReader(input), &got)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(got.Pieces) != "spam" {
		t.Fatalf("got %q, want %q", string(got.Pieces), "spam")
	}
}

func TestUnmarshalSlice(t *testing.T) {
	input := "li1ei2ei3ee"

	var got []int
	err := Unmarshal(strings.NewReader(input), &got)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("got len %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestUnmarshalMap(t *testing.T) {
	input := "d3:fooi42e3:bari7ee"
	wantFoo := 42
	wantBar := 7

	var got map[string]int
	err := Unmarshal(strings.NewReader(input), &got)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got["foo"] != wantFoo {
		t.Fatalf("got foo=%d, want 42", got["foo"])
	}
	if got["bar"] != wantBar {
		t.Fatalf("got bar=%d, want 7", got["bar"])
	}
}

func TestUnmarshalMapAny(t *testing.T) {
	input := "d3:numi42e4:text4:spame"

	var got map[string]any
	err := Unmarshal(strings.NewReader(input), &got)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n, ok := got["num"].(int64)
	if !ok || n != 42 {
		t.Fatalf("got num=%#v, want int64(42)", got["num"])
	}

	b, ok := got["text"].([]byte)
	if !ok || string(b) != "spam" {
		t.Fatalf("got text=%#v, want []byte(\"spam\")", got["text"])
	}
}

func TestUnmarshalRequiresPointer(t *testing.T) {
	type S struct {
		Name string `bencode:"name"`
	}

	input := "d4:name4:spame"

	var got S
	err := Unmarshal(strings.NewReader(input), got)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestUnmarshalNilPointer(t *testing.T) {
	var got *struct {
		Name string `bencode:"name"`
	}

	err := Unmarshal(strings.NewReader("d4:name4:spame"), got)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestUnmarshalTypeMismatch(t *testing.T) {
	type S struct {
		Name int `bencode:"name"`
	}

	input := "d4:name4:spame"

	var got S
	err := Unmarshal(strings.NewReader(input), &got)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestUnmarshalNegativeIntoUnsigned(t *testing.T) {
	type S struct {
		N uint `bencode:"n"`
	}

	input := "d1:ni-3ee"

	var got S
	err := Unmarshal(strings.NewReader(input), &got)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestDecodeInto(t *testing.T) {
	type S struct {
		Name string `bencode:"name"`
	}

	dec := NewDecoder(strings.NewReader("d4:name4:spame"))

	var got S

	err := dec.DecodeInto(&got)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Name != "spam" {
		t.Fatalf("got %q, want %q", got.Name, "spam")
	}
}

func TestUnmarshalStructFieldNameFallback(t *testing.T) {
	type Info struct {
		Name string `bencode:"name"`
	}

	type Torrent struct {
		Info Info
	}

	var got Torrent
	err := Unmarshal(strings.NewReader("d4:infod4:name4:spamee"), &got)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Info.Name != "spam" {
		t.Fatalf("got info.name %q, want %q", got.Info.Name, "spam")
	}
}

func TestUnmarshalUintField(t *testing.T) {
	type S struct {
		N uint `bencode:"n"`
	}

	var got S
	err := Unmarshal(strings.NewReader("d1:ni7ee"), &got)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.N != 7 {
		t.Fatalf("got %d, want %d", got.N, 7)
	}
}
