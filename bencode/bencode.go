package bencode

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

type Decoder struct {
	br *bufio.Reader
}

type Encoder struct {
	bw *bufio.Writer
}

func NewDecoder(r io.Reader) *Decoder {
	if br, ok := r.(*bufio.Reader); ok {
		return &Decoder{br}
	}

	return &Decoder{br: bufio.NewReader(r)}
}

func NewEncoder(w io.Writer) *Encoder {
	if bw, ok := w.(*bufio.Writer); ok {
		return &Encoder{bw}
	}

	return &Encoder{bw: bufio.NewWriter(w)}
}

type Value interface{}

type Dict map[string]Value
type List []Value

func (d *Decoder) decodeValue() (Value, error) {
	b, err := d.br.Peek(1)
	if err != nil {
		return nil, err
	}

	switch b[0] {
	case 'i':
		return d.decodeInt()
	case 'l':
		return d.decodeList()
	case 'd':
		return d.decodeDict()
	default:
		if b[0] >= '0' && b[0] <= '9' {
			return d.decodeBytes()
		}
		return nil, fmt.Errorf("bencode: unexpected byte %q", b[0])
	}
}

func (d *Decoder) decodeInt() (int64, error) {
	// consume 'i'
	if err := expectByte(d.br, 'i'); err != nil {
		return 0, err
	}

	sign := int64(1)
	b, err := d.br.ReadByte()
	if err != nil {
		return 0, err
	}

	if b == '-' {
		sign = -1
		b, err = d.br.ReadByte()
		if err != nil {
			return 0, err
		}
	}

	if b == 'e' {
		return 0, errors.New("bencode: empty integer")
	}

	if b < '0' || b > '9' {
		return 0, fmt.Errorf("bencode: invalid integer byte %q", b)
	}

	// leading zero not allowed
	if b == '0' {
		next, err := d.br.ReadByte()
		if err != nil {
			return 0, err
		}
		if next != 'e' {
			return 0, errors.New("bencode: leading zero in integer")
		}
		if sign == -1 {
			return 0, errors.New("bencode: negative zero is invalid")
		}

		return 0, nil
	}

	var n int64
	for {
		if b < '0' || b > '9' {
			return 0, fmt.Errorf("bencode: invalid integer byte %q", b)
		}
		n = n*10 + int64(b-'0')

		b, err = d.br.ReadByte()
		if err != nil {
			return 0, err
		}

		if b == 'e' {
			break
		}
	}

	return sign * n, nil
}

func (d *Decoder) decodeBytes() ([]byte, error) {
	n, err := d.readStringLen()
	if err != nil {
		return nil, err
	}

	buf := make([]byte, n)
	if _, err := io.ReadFull(d.br, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (d *Decoder) readStringLen() (int, error) {
	b, err := d.br.ReadByte()
	if err != nil {
		return 0, err
	}

	if b < '0' || b > '9' {
		return 0, fmt.Errorf("bencode: invalid string length start %q", b)
	}

	if b == '0' {
		colon, err := d.br.ReadByte()
		if err != nil {
			return 0, err
		}

		if colon != ':' {
			return 0, errors.New("bencode: expected ':' after string length")
		}
		return 0, nil
	}

	n := int(b - '0')
	for {
		b, err := d.br.ReadByte()
		if err != nil {
			return 0, err
		}
		if b == ':' {
			break
		}

		if b < '0' || b > '9' {
			return 0, fmt.Errorf("bencode: invalid string length byte %q", b)
		}

		n = n*10 + int(b-'0')
	}

	return n, nil
}

func (d *Decoder) decodeList() (List, error) {
	if err := expectByte(d.br, 'l'); err != nil {
		return nil, err
	}

	out := make(List, 0)
	for {
		b, err := d.br.Peek(1)
		if err != nil {
			return nil, err
		}

		if b[0] == 'e' {
			_, _ = d.br.ReadByte()
			break
		}

		v, err := d.decodeValue()
		if err != nil {
			return nil, err
		}

		out = append(out, v)
	}
	return out, nil
}

func (d *Decoder) decodeDict() (Dict, error) {
	if err := expectByte(d.br, 'd'); err != nil {
		return nil, err
	}

	out := make(Dict)
	for {
		b, err := d.br.Peek(1)
		if err != nil {
			return nil, err
		}

		if b[0] == 'e' {
			_, _ = d.br.ReadByte()
			break
		}

		keyBytes, err := d.decodeBytes()
		if err != nil {
			return nil, err
		}

		key := string(keyBytes)

		val, err := d.decodeValue()
		if err != nil {
			return nil, err
		}

		out[key] = val
	}
	return out, nil
}

func expectByte(br *bufio.Reader, want byte) error {
	got, err := br.ReadByte()
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("bencode: expected %q, got %q", want, got)
	}
	return nil
}

// Decode decodes a bencoded value
func (d *Decoder) Decode() (Value, error) {
	v, err := d.decodeValue()
	if err != nil {
		return nil, err
	}

	_, err = d.br.Peek(1)
	if errors.Is(err, io.EOF) {
		return v, nil
	}
	if err != nil {
		return nil, err
	}

	return nil, errors.New("bencode: trailing data after top-level value")
}

// Unmarshal is a convenience function that wraps byte data in a reader.
func Unmarshal(data []byte) (Value, error) {
	return NewDecoder(bytes.NewReader(data)).Decode()
}
