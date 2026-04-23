package bencode

import (
	"bytes"
	"reflect"
	"testing"
)

func TestEncoder_Encode_Primitives(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"string", "spam", "4:spam"},
		{"bytes", []byte("spam"), "4:spam"},
		{"int", int64(42), "i42e"},
		{"negative int", int64(-7), "i-7e"},
		{"bool true", true, "i1e"},
		{"bool false", false, "i0e"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)

			if err := enc.Encode(tc.in); err != nil {
				t.Fatalf("Encode() error = %v", err)
			}

			if got := buf.String(); got != tc.want {
				t.Fatalf("Encode() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEncoder_Encode_List(t *testing.T) {
	in := []any{
		[]byte("spam"),
		int64(42),
		[]byte("eggs"),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	if err := enc.Encode(in); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	want := "l4:spami42e4:eggse"
	if got := buf.String(); got != want {
		t.Fatalf("Encode() = %q, want %q", got, want)
	}
}

func TestEncoder_Encode_Map_SortsKeys(t *testing.T) {
	in := map[string]any{
		"cow":  []byte("moo"),
		"spam": []byte("eggs"),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	if err := enc.Encode(in); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	want := "d3:cow3:moo4:spam4:eggse"
	if got := buf.String(); got != want {
		t.Fatalf("Encode() = %q, want %q", got, want)
	}
}

func TestEncoder_Encode_Struct(t *testing.T) {
	type payload struct {
		Announce string `bencode:"announce"`
		Length   int64  `bencode:"length"`
		private  string
		Ignore   string `bencode:"-"`
	}

	in := payload{
		Announce: "http://tracker",
		Length:   123,
		private:  "hidden",
		Ignore:   "skip",
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	if err := enc.Encode(in); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	want := "d8:announce14:http://tracker6:lengthi123ee"
	if got := buf.String(); got != want {
		t.Fatalf("Encode() = %q, want %q", got, want)
	}
}

func TestEncoder_Encode_Decode_RoundTrip(t *testing.T) {
	in := Dict{
		"announce": []byte("http://tracker"),
		"length":   int64(123),
		"pieces":   []byte{1, 2, 3, 4},
		"files": List{
			Dict{
				"path":   []byte("a.txt"),
				"length": int64(5),
			},
		},
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	if err := enc.Encode(in); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got, err := dec.Decode()
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round trip mismatch\ngot  %#v\nwant %#v", got, in)
	}
}

func TestEncoder_Encode_UnsupportedType(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	err := enc.Encode(3.14)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestEncoder_Encode_MapKeyTypeError(t *testing.T) {
	in := map[int]string{
		1: "a",
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	err := enc.Encode(in)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
