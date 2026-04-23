package bencode

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode"
)

var (
	ErrSyntax         = errors.New("bencode: syntax error")
	ErrInvalidInteger = errors.New("bencode: invalid integer")
	ErrInvalidString  = errors.New("bencode: invalid byte string")
	ErrTrailingData   = errors.New("bencode: trailing data after top-level value")

	ErrNilDestination     = errors.New("bencode: nil destination")
	ErrInvalidDestination = errors.New("bencode: destination must be a non-nil pointer")
	ErrUnsettableValue    = errors.New("bencode: destination cannot be set")
	ErrInvalidValue       = errors.New("bencode: invalid destination value")

	ErrTypeMismatch    = errors.New("bencode: type mismatch")
	ErrUnsupportedType = errors.New("bencode: unsupported destination type")
	ErrMapKeyType      = errors.New("bencode: map key type must be string")
	ErrOverflow        = errors.New("bencode: numeric overflow")
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
		return nil, fmt.Errorf("%w: unexpected byte %q", ErrSyntax, b[0])
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
		return 0, fmt.Errorf("%w: empty integer", ErrInvalidInteger)
	}

	if b < '0' || b > '9' {
		return 0, fmt.Errorf("%w: invalid integer byte %q", ErrInvalidInteger, b)
	}

	// leading zero not allowed
	if b == '0' {
		next, err := d.br.ReadByte()
		if err != nil {
			return 0, err
		}
		if next != 'e' {
			return 0, fmt.Errorf("%w: leading zero in integer", ErrInvalidInteger)
		}
		if sign == -1 {
			return 0, fmt.Errorf("%w: negative zero is invalid", ErrInvalidInteger)
		}

		return 0, nil
	}

	var n int64
	for {
		if b < '0' || b > '9' {
			return 0, fmt.Errorf("%w: invalid integer byte %q", ErrInvalidInteger, b)
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
		return 0, fmt.Errorf("%w: invalid string length start %q", ErrInvalidInteger, b)
	}

	if b == '0' {
		colon, err := d.br.ReadByte()
		if err != nil {
			return 0, err
		}

		if colon != ':' {
			return 0, fmt.Errorf("%w: expected ':' after string length", ErrInvalidString)
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
			return 0, fmt.Errorf("%w: invalid string length byte %q", ErrInvalidString, b)
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
		return fmt.Errorf("%w: expected %q, got %q", ErrSyntax, want, got)
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

	return nil, ErrTrailingData
}

func (d *Decoder) DecodeInto(v any) error {
	if v == nil {
		return ErrNilDestination
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return ErrInvalidDestination
	}

	src, err := d.Decode()
	if err != nil {
		return err
	}

	return assignValue(rv.Elem(), src)
}

func assignValue(dst reflect.Value, src any) error {
	if !dst.IsValid() {
		return ErrInvalidValue
	}

	if !dst.CanSet() {
		return ErrUnsettableValue
	}

	if dst.Kind() == reflect.Pointer {
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		return assignValue(dst.Elem(), src)
	}

	switch dst.Kind() {
	case reflect.Interface:
		dst.Set(reflect.ValueOf(src))
		return nil

	case reflect.String:
		b, ok := src.([]byte)
		if !ok {
			return fmt.Errorf("%w: cannot assign %T to %s", ErrTypeMismatch, src, dst.Type())
		}
		dst.SetString(string(b))
		return nil

	case reflect.Slice:
		if dst.Type().Elem().Kind() == reflect.Uint8 {
			b, ok := src.([]byte)
			if !ok {
				return fmt.Errorf("%w: cannot assign %T to %s", ErrTypeMismatch, src, dst.Type())
			}
			dst.SetBytes(b)
			return nil
		}

		list, ok := src.(List)
		if !ok {
			return fmt.Errorf("%w: cannot assign %T to %s", ErrTypeMismatch, src, dst.Type())
		}

		out := reflect.MakeSlice(dst.Type(), len(list), len(list))
		for i := range list {
			if err := assignValue(out.Index(i), list[i]); err != nil {
				return fmt.Errorf("slice index %d: %w", i, err)
			}
		}
		dst.Set(out)
		return nil

	case reflect.Array:
		if dst.Type().Elem().Kind() == reflect.Uint8 {
			b, ok := src.([]byte)
			if !ok {
				return fmt.Errorf("%w: cannot assign %T to %s", ErrTypeMismatch, src, dst.Type())
			}
			if len(b) != dst.Len() {
				return fmt.Errorf("%w: byte array length mismatch: got %d want %d", ErrTypeMismatch, len(b), dst.Len())
			}
			reflect.Copy(dst, reflect.ValueOf(b))
			return nil
		}

		list, ok := src.(List)
		if !ok {
			return fmt.Errorf("%w: cannot assign %T to %s", ErrTypeMismatch, src, dst.Type())
		}
		if len(list) != dst.Len() {
			return fmt.Errorf("%w: array length mismatch: got %d want %d", ErrTypeMismatch, len(list), dst.Len())
		}
		for i := range list {
			if err := assignValue(dst.Index(i), list[i]); err != nil {
				return fmt.Errorf("array index %d: %w", i, err)
			}
		}
		return nil

	case reflect.Map:
		dict, ok := src.(Dict)
		if !ok {
			return fmt.Errorf("%w: cannot assign %T to %s", ErrTypeMismatch, src, dst.Type())
		}

		if dst.Type().Key().Kind() != reflect.String {
			return ErrMapKeyType
		}

		if dst.IsNil() {
			dst.Set(reflect.MakeMap(dst.Type()))
		}

		elemType := dst.Type().Elem()
		for k, v := range dict {
			elem := reflect.New(elemType).Elem()
			if err := assignValue(elem, v); err != nil {
				return fmt.Errorf("map key %q: %w", k, err)
			}
			dst.SetMapIndex(reflect.ValueOf(k), elem)
		}
		return nil

	case reflect.Struct:
		dict, ok := src.(Dict)
		if !ok {
			return fmt.Errorf("%w: cannot assign %T to %s", ErrTypeMismatch, src, dst.Type())
		}
		return assignStruct(dst, dict)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, ok := src.(int64)
		if !ok {
			return fmt.Errorf("%w: cannot assign %T to %s", ErrTypeMismatch, src, dst.Type())
		}
		if dst.OverflowInt(n) {
			return fmt.Errorf("%w: assigning %d to %s", ErrOverflow, n, dst.Type())
		}
		dst.SetInt(n)
		return nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, ok := src.(int64)
		if !ok {
			return fmt.Errorf("%w: cannot assign %T to %s", ErrTypeMismatch, src, dst.Type())
		}
		if n < 0 {
			return fmt.Errorf("%w: cannot assign negative integer %d to %s", ErrTypeMismatch, n, dst.Type())
		}
		if dst.OverflowUint(uint64(n)) {
			return fmt.Errorf("%w: assigning %d to %s", ErrOverflow, n, dst.Type())
		}
		dst.SetUint(uint64(n))
		return nil

	case reflect.Bool:
		n, ok := src.(int64)
		if !ok {
			return fmt.Errorf("%w: cannot assign %T to %s", ErrTypeMismatch, src, dst.Type())
		}
		dst.SetBool(n != 0)
		return nil
	}
	return fmt.Errorf("%w: %s", ErrUnsupportedType, dst.Type())
}

func assignStruct(dst reflect.Value, dict Dict) error {
	t := dst.Type()

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)

		// skip unexported fields
		if sf.PkgPath != "" {
			continue
		}

		tag := sf.Tag.Get("bencode")
		switch tag {
		case "-":
			continue
		}

		src, ok := lookupStructFieldValue(dict, sf, tag)
		if !ok {
			continue
		}

		if err := assignValue(dst.Field(i), src); err != nil {
			return fmt.Errorf("field %s: %w", sf.Name, err)
		}
	}
	return nil
}

func lookupStructFieldValue(dict Dict, sf reflect.StructField, tag string) (any, bool) {
	if tag != "" {
		src, ok := dict[tag]
		return src, ok
	}

	for _, candidate := range fieldNameCandidates(sf.Name) {
		if src, ok := dict[candidate]; ok {
			return src, true
		}
	}

	return nil, false
}

func fieldNameCandidates(name string) []string {
	words := splitFieldName(name)
	candidates := []string{
		name,
		lowerFirst(name),
		strings.Join(words, " "),
		strings.Join(words, "-"),
		strings.Join(words, "_"),
		strings.Join(words, ""),
	}

	seen := make(map[string]struct{}, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}

	return out
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}

	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func splitFieldName(name string) []string {
	if name == "" {
		return nil
	}

	runes := []rune(name)
	start := 0
	words := make([]string, 0, len(runes))

	for i := 1; i < len(runes); i++ {
		curr := runes[i]
		prev := runes[i-1]

		if !unicode.IsUpper(curr) {
			continue
		}

		nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
		if unicode.IsLower(prev) || nextIsLower {
			words = append(words, strings.ToLower(string(runes[start:i])))
			start = i
		}
	}

	words = append(words, strings.ToLower(string(runes[start:])))
	return words
}
