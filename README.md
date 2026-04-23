# go-torrent

This is my attempt at creating a simple torrent client in Go.

## Status

*Completed*:

- [x] Bencode Decoding/Encoding
- [x] Torrent File Parse/Extract to struct
- [x] Calculate Info Hash

**In Progress/Next Steps**:

- [ ] Tracker connection
- [ ] Peer messaging protocol/struct definitions

## Usage

### Bencode

The `bencode` package can decode raw bencode values into Go values, unmarshal
directly into structs, and encode Go values back to bencode.

Import the package:

```go
import "github.com/elliota43/gotorrent/bencode"
```

#### Decode a raw value

Use `NewDecoder` when you want to inspect bencode without defining a struct
first:

```go
r := strings.NewReader("d3:cow3:moo4:spam4:eggse")

value, err := bencode.NewDecoder(r).Decode()
if err != nil {
	return err
}

dict := value.(bencode.Dict)
fmt.Printf("%s\n", dict["cow"].([]byte)) // moo
```

Decoded values use these Go types:

- Integers decode as `int64`
- Byte strings decode as `[]byte`
- Lists decode as `bencode.List`
- Dictionaries decode as `bencode.Dict`

#### Unmarshal into a struct

Use `Unmarshal` or `Decoder.DecodeInto` when you know the expected shape:

```go
type Info struct {
	Name        string `bencode:"name"`
	PieceLength int    `bencode:"piece length"`
	Pieces      []byte `bencode:"pieces"`
}

type Torrent struct {
	Announce string `bencode:"announce"`
	Info     Info   `bencode:"info"`
}

input := "d8:announce14:http://tracker4:infod4:name8:test.iso12:piece lengthi16384e6:pieces4:abcdee"

var torrent Torrent
if err := bencode.Unmarshal(strings.NewReader(input), &torrent); err != nil {
	return err
}

fmt.Println(torrent.Announce)
fmt.Println(torrent.Info.Name)
```

Struct fields must be exported. The `bencode` tag maps a field to a dictionary
key, and `bencode:"-"` skips a field. If a field has no tag, the decoder tries
common field-name forms such as `InfoHash`, `infoHash`, `info hash`,
`info-hash`, and `info_hash`.

For torrent metainfo, the decoder can also keep the raw bytes for a dictionary
value or calculate its SHA-1 hash:

```go
type Torrent struct {
	Info      Info     `bencode:"info"`
	InfoBytes []byte   `bencode:"info,raw"`
	InfoHash  [20]byte `bencode:"info,sha1"`
}
```

`raw` and `sha1` are useful for the torrent `info` dictionary because the info
hash must be calculated from the exact original bencoded bytes.

#### Encode a value

Use `NewEncoder` to write bencode:

```go
var buf bytes.Buffer

err := bencode.NewEncoder(&buf).Encode(map[string]any{
	"cow":  []byte("moo"),
	"spam": []byte("eggs"),
})
if err != nil {
	return err
}

fmt.Println(buf.String()) // d3:cow3:moo4:spam4:eggse
```

The encoder supports strings, `[]byte`, integers, unsigned integers that fit in
an `int64`, booleans, slices, arrays, maps with string keys, and structs.
