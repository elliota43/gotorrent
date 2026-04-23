package bencode

import (
	"bufio"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
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
	br  *bufio.Reader
	raw []byte
}

type Encoder struct {
	bw *bufio.Writer
}

func NewDecoder(r io.Reader) *Decoder {
	if br, ok := r.(*bufio.Reader); ok {
		return &Decoder{br: br}
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

type rawValue struct {
	value Value
	raw   []byte
}

func (e *Encoder) Encode(v any) error {
	if err := e.encodeValue(reflect.ValueOf(v)); err != nil {
		return err
	}
	return e.bw.Flush()
}

func (e *Encoder) encodeValue(v reflect.Value) error {
	if !v.IsValid() {
		return ErrUnsupportedType
	}

	for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return fmt.Errorf("%w: nil %s", ErrUnsupportedType, v.Type())
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.String:
		return e.encodeBytes([]byte(v.String()))

	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return e.encodeBytes(v.Bytes())
		}
		return e.encodeSliceLike(v)

	case reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			buf := make([]byte, v.Len())
			reflect.Copy(reflect.ValueOf(buf), v)
			return e.encodeBytes(buf)
		}
		return e.encodeSliceLike(v)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return e.encodeInt(v.Int())

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u := v.Uint()
		if u > uint64(^uint64(0)>>1) {
			return fmt.Errorf("%w: uint too large for bencode int: %d", ErrOverflow, u)
		}
		return e.encodeInt(int64(u))

	case reflect.Bool:
		if v.Bool() {
			return e.encodeInt(1)
		}
		return e.encodeInt(0)

	case reflect.Map:
		return e.encodeMap(v)

	case reflect.Struct:
		return e.encodeStruct(v)

	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedType, v.Type())
	}
}

func (e *Encoder) encodeInt(n int64) error {
	if err := e.writeByte('i'); err != nil {
		return err
	}

	if _, err := e.writeString(strconv.FormatInt(n, 10)); err != nil {
		return err
	}

	return e.writeByte('e')
}

func (e *Encoder) encodeBytes(b []byte) error {
	if _, err := e.writeString(strconv.Itoa(len(b))); err != nil {
		return err
	}

	if err := e.writeByte(':'); err != nil {
		return err
	}

	_, err := e.bw.Write(b)
	return err
}

func (e *Encoder) encodeSliceLike(v reflect.Value) error {
	if err := e.writeByte('l'); err != nil {
		return err
	}

	for i := 0; i < v.Len(); i++ {
		if err := e.encodeValue(v.Index(i)); err != nil {
			return err
		}
	}
	return e.writeByte('e')
}

func (e *Encoder) encodeMap(v reflect.Value) error {
	if v.Type().Key().Kind() != reflect.String {
		return ErrMapKeyType
	}

	if err := e.writeByte('d'); err != nil {
		return err
	}

	keys := v.MapKeys()
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})

	for _, k := range keys {
		if err := e.encodeBytes([]byte(k.String())); err != nil {
			return err
		}
		if err := e.encodeValue(v.MapIndex(k)); err != nil {
			return err
		}
	}

	return e.writeByte('e')
}

func (e *Encoder) encodeStruct(v reflect.Value) error {
	if err := e.writeByte('d'); err != nil {
		return err
	}

	type fieldEntry struct {
		key string
		val reflect.Value
	}

	t := v.Type()
	fields := make([]fieldEntry, 0, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)

		if sf.PkgPath != "" {
			continue
		}

		name, opts := parseTag(sf.Tag.Get("bencode"))
		if name == "-" {
			continue
		}
		if opts["raw"] || opts["sha1"] {
			continue
		}

		key := name
		if key == "" {
			key = lowerFirst(sf.Name)
		}

		fields = append(fields, fieldEntry{
			key: key,
			val: v.Field(i),
		})
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].key < fields[j].key
	})

	for _, f := range fields {
		if err := e.encodeBytes([]byte(f.key)); err != nil {
			return err
		}

		if err := e.encodeValue(f.val); err != nil {
			return fmt.Errorf("field %s: %w", f.key, err)
		}
	}

	return e.writeByte('e')
}

func (e *Encoder) writeByte(b byte) error {
	return e.bw.WriteByte(b)
}

func (e *Encoder) writeString(s string) (int, error) {
	return e.bw.WriteString(s)
}

func (d *Decoder) decodeValue() (Value, error) {
	start := len(d.raw)

	b, err := d.br.Peek(1)
	if err != nil {
		return nil, err
	}

	var v Value
	switch b[0] {
	case 'i':
		v, err = d.decodeInt()
	case 'l':
		v, err = d.decodeList()
	case 'd':
		v, err = d.decodeDict()
	default:
		if b[0] >= '0' && b[0] <= '9' {
			v, err = d.decodeBytes()
			break
		}
		return nil, fmt.Errorf("%w: unexpected byte %q", ErrSyntax, b[0])
	}
	if err != nil {
		return nil, err
	}

	return rawValue{value: v, raw: d.raw[start:len(d.raw)]}, nil
}

func (d *Decoder) decodeInt() (int64, error) {
	// consume 'i'
	if err := d.expectByte('i'); err != nil {
		return 0, err
	}

	sign := int64(1)
	b, err := d.readByte()
	if err != nil {
		return 0, err
	}

	if b == '-' {
		sign = -1
		b, err = d.readByte()
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
		next, err := d.readByte()
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

		b, err = d.readByte()
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
	if err := d.readFull(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (d *Decoder) readStringLen() (int, error) {
	b, err := d.readByte()
	if err != nil {
		return 0, err
	}

	if b < '0' || b > '9' {
		return 0, fmt.Errorf("%w: invalid string length start %q", ErrInvalidInteger, b)
	}

	if b == '0' {
		colon, err := d.readByte()
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
		b, err := d.readByte()
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
	if err := d.expectByte('l'); err != nil {
		return nil, err
	}

	out := make(List, 0)
	for {
		b, err := d.br.Peek(1)
		if err != nil {
			return nil, err
		}

		if b[0] == 'e' {
			_, _ = d.readByte()
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
	if err := d.expectByte('d'); err != nil {
		return nil, err
	}

	out := make(Dict)
	for {
		b, err := d.br.Peek(1)
		if err != nil {
			return nil, err
		}

		if b[0] == 'e' {
			_, _ = d.readByte()
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

func (d *Decoder) readByte() (byte, error) {
	b, err := d.br.ReadByte()
	if err != nil {
		return 0, err
	}

	d.raw = append(d.raw, b)
	return b, nil
}

func (d *Decoder) readFull(buf []byte) error {
	n, err := io.ReadFull(d.br, buf)
	if n > 0 {
		d.raw = append(d.raw, buf[:n]...)
	}
	return err
}

func (d *Decoder) expectByte(want byte) error {
	got, err := d.readByte()
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

	if err := d.checkTrailing(); err != nil {
		return nil, err
	}

	return unwrapRawValue(v), nil
}

func (d *Decoder) DecodeInto(v any) error {
	if v == nil {
		return ErrNilDestination
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return ErrInvalidDestination
	}

	src, err := d.decodeValue()
	if err != nil {
		return err
	}
	if err := d.checkTrailing(); err != nil {
		return err
	}

	return assignValue(rv.Elem(), src)
}

func (d *Decoder) checkTrailing() error {
	_, err := d.br.Peek(1)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}

	return ErrTrailingData
}

func unwrapRawValue(src any) any {
	rv, ok := src.(rawValue)
	if ok {
		src = rv.value
	}

	switch v := src.(type) {
	case Dict:
		out := make(Dict, len(v))
		for key, val := range v {
			out[key] = unwrapRawValue(val)
		}
		return out
	case List:
		out := make(List, len(v))
		for i, val := range v {
			out[i] = unwrapRawValue(val)
		}
		return out
	default:
		return src
	}
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

	if rv, ok := src.(rawValue); ok {
		src = rv.value
	}

	switch dst.Kind() {
	case reflect.Interface:
		dst.Set(reflect.ValueOf(unwrapRawValue(src)))
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

		name, opts := parseTag(sf.Tag.Get("bencode"))
		if name == "-" {
			continue
		}

		src, ok := lookupStructFieldValue(dict, sf, name)
		if !ok {
			continue
		}

		var err error
		switch {
		case opts["raw"]:
			err = assignRawBytes(dst.Field(i), src)
		case opts["sha1"]:
			err = assignSHA1(dst.Field(i), src)
		default:
			err = assignValue(dst.Field(i), src)
		}
		if err != nil {
			return fmt.Errorf("field %s: %w", sf.Name, err)
		}
	}
	return nil
}

func parseTag(tag string) (string, map[string]bool) {
	if tag == "" {
		return "", nil
	}

	parts := strings.Split(tag, ",")
	name := parts[0]
	opts := make(map[string]bool, len(parts)-1)
	for _, opt := range parts[1:] {
		if opt != "" {
			opts[opt] = true
		}
	}

	return name, opts
}

func assignRawBytes(dst reflect.Value, src any) error {
	rv, ok := src.(rawValue)
	if !ok {
		return fmt.Errorf("%w: raw bytes unavailable for %T", ErrTypeMismatch, src)
	}

	return assignBytes(dst, rv.raw)
}

func assignSHA1(dst reflect.Value, src any) error {
	rv, ok := src.(rawValue)
	if !ok {
		return fmt.Errorf("%w: raw bytes unavailable for %T", ErrTypeMismatch, src)
	}

	sum := sha1.Sum(rv.raw)
	return assignBytes(dst, sum[:])
}

func assignBytes(dst reflect.Value, b []byte) error {
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
		return assignBytes(dst.Elem(), b)
	}

	switch dst.Kind() {
	case reflect.Slice:
		if dst.Type().Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("%w: cannot assign raw bytes to %s", ErrTypeMismatch, dst.Type())
		}
		dst.SetBytes(append([]byte(nil), b...))
		return nil
	case reflect.Array:
		if dst.Type().Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("%w: cannot assign raw bytes to %s", ErrTypeMismatch, dst.Type())
		}
		if len(b) != dst.Len() {
			return fmt.Errorf("%w: byte array length mismatch: got %d want %d", ErrTypeMismatch, len(b), dst.Len())
		}
		reflect.Copy(dst, reflect.ValueOf(b))
		return nil
	}

	return fmt.Errorf("%w: cannot assign raw bytes to %s", ErrUnsupportedType, dst.Type())
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
