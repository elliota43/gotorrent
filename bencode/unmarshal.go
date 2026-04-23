package bencode

import "io"

func Unmarshal(r io.Reader, v any) error {
	dec := NewDecoder(r)
	return dec.DecodeInto(v)
}
