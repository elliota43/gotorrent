package torrent

import (
	"bytes"
	"crypto/sha1"
	"strings"
	"testing"
)

func TestInfoDictionary_IsSingleAndMultiFile(t *testing.T) {
	single := InfoDictionary{
		PieceLength: 16384,
		Pieces:      make([]byte, sha1.Size),
		Name:        "file.txt",
		Length:      123,
	}

	if !single.IsSingleFile() {
		t.Fatalf("expected single-file torrent")
	}

	if single.IsMultiFile() {
		t.Fatalf("did not expect multi-file torrent")
	}

	multi := InfoDictionary{
		PieceLength: 16384,
		Pieces:      make([]byte, sha1.Size),
		Name:        "my-folder",
		Files: []FileInfo{
			{Length: 10, Path: []string{"a.txt"}},
			{Length: 20, Path: []string{"dir", "b.txt"}},
		},
	}

	if multi.IsSingleFile() {
		t.Fatalf("did not expect single-file torrent")
	}
	if !multi.IsMultiFile() {
		t.Fatalf("expected multi-file torrent")
	}
}

func TestInfoDictionary_TotalLength(t *testing.T) {
	single := InfoDictionary{
		Length: 999,
	}
	if got := single.TotalLength(); got != 999 {
		t.Fatalf("single-file total length: got %d want %d", got, 999)
	}

	multi := InfoDictionary{
		Files: []FileInfo{
			{Length: 10, Path: []string{"a.txt"}},
			{Length: 20, Path: []string{"b.txt"}},
			{Length: 30, Path: []string{"c.txt"}},
		},
	}
	if got := multi.TotalLength(); got != 60 {
		t.Fatalf("multi-file total length: got %d want %d", got, 60)
	}
}

func TestInfoDictionary_PieceCount(t *testing.T) {
	i := InfoDictionary{
		Pieces: bytes.Repeat([]byte{0xAB}, sha1.Size*3),
	}

	if got := i.PieceCount(); got != 3 {
		t.Fatalf("got %d want %d", got, 3)
	}
}

func TestInfoDictionary_Validate_SingleFile_OK(t *testing.T) {
	i := InfoDictionary{
		PieceLength: 16384,
		Pieces:      make([]byte, sha1.Size*2),
		Name:        "file.txt",
		Length:      32768,
	}

	if err := i.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInfoDictionary_Validate_MultiFile_OK(t *testing.T) {
	i := InfoDictionary{
		PieceLength: 16384,
		Pieces:      make([]byte, sha1.Size),
		Name:        "folder",
		Files: []FileInfo{
			{Length: 12, Path: []string{"a.txt"}},
			{Length: 34, Path: []string{"dir", "b.txt"}},
		},
	}

	if err := i.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInfoDictionary_Validate_BadPieceLength(t *testing.T) {
	i := InfoDictionary{
		PieceLength: 0,
		Pieces:      make([]byte, sha1.Size),
		Length:      1,
	}

	if err := i.Validate(); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestInfoDictionary_Validate_BadPiecesFieldLength(t *testing.T) {
	i := InfoDictionary{
		PieceLength: 16384,
		Pieces:      []byte{1, 2, 3},
		Length:      1,
	}

	if err := i.Validate(); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestInfoDictionary_Validate_SingleFileMissingLength(t *testing.T) {
	i := InfoDictionary{
		PieceLength: 16384,
		Pieces:      make([]byte, sha1.Size),
		Name:        "file.txt",
		Length:      0,
	}

	if err := i.Validate(); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestInfoDictionary_Validate_MultiFileEmptyPath(t *testing.T) {
	i := InfoDictionary{
		PieceLength: 16384,
		Pieces:      make([]byte, sha1.Size),
		Name:        "folder",
		Files: []FileInfo{
			{Length: 12, Path: nil},
		},
	}

	if err := i.Validate(); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestInfoDictionary_PieceHashes(t *testing.T) {
	raw := make([]byte, sha1.Size*2)
	for i := 0; i < sha1.Size; i++ {
		raw[i] = byte(i)
		raw[sha1.Size+i] = byte(100 + i)
	}

	i := InfoDictionary{
		Pieces: raw,
	}

	hashes, err := i.PieceHashes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(hashes) != 2 {
		t.Fatalf("got %d hashes want %d", len(hashes), 2)
	}

	for j := 0; j < sha1.Size; j++ {
		if hashes[0][j] != byte(j) {
			t.Fatalf("first hash byte %d: got %d want %d", j, hashes[0][j], byte(j))
		}
		if hashes[1][j] != byte(100+j) {
			t.Fatalf("second hash byte %d: got %d want %d", j, hashes[1][j], byte(100+j))
		}
	}
}

func TestInfoDictionary_PieceHashes_BadLength(t *testing.T) {
	i := InfoDictionary{
		Pieces: []byte{1, 2, 3},
	}

	if _, err := i.PieceHashes(); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestOpen_SingleFileTorrent(t *testing.T) {
	pieces := strings.Repeat("a", sha1.Size)

	input := "" +
		"d" +
		"8:announce" + "14:http://tracker" +
		"4:info" +
		"d" +
		"6:length" + "i123e" +
		"4:name" + "8:file.txt" +
		"12:piece length" + "i16384e" +
		"6:pieces" + "20:" + pieces +
		"e" +
		"e"

	meta, err := Open(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta.Announce != "http://tracker" {
		t.Fatalf("announce: got %q want %q", meta.Announce, "http://tracker")
	}

	if meta.Info.Name != "file.txt" {
		t.Fatalf("name: got %q want %q", meta.Info.Name, "file.txt")
	}
	if meta.Info.Length != 123 {
		t.Fatalf("length: got %d want %d", meta.Info.Length, 123)
	}
	if meta.Info.PieceLength != 16384 {
		t.Fatalf("piece length: got %d want %d", meta.Info.PieceLength, 16384)
	}
	if len(meta.Info.Pieces) != sha1.Size {
		t.Fatalf("pieces len: got %d want %d", len(meta.Info.Pieces), sha1.Size)
	}

	if !meta.Info.IsSingleFile() {
		t.Fatalf("expected single-file torrent")
	}
}

func TestOpen_MultiFileTorrent(t *testing.T) {
	pieces := strings.Repeat("b", sha1.Size)

	input := "" +
		"d" +
		"8:announce" + "14:http://tracker" +
		"4:info" +
		"d" +
		"4:name" + "6:mydir!" +
		"12:piece length" + "i16384e" +
		"6:pieces" + "20:" + pieces +
		"5:files" +
		"l" +
		"d" +
		"6:length" + "i10e" +
		"4:path" + "l" + "5:a.txte" +
		"e" +
		"d" +
		"6:length" + "i20e" +
		"4:path" + "l" + "3:dir" + "5:b.txte" +
		"e" +
		"e" +
		"e" +
		"e"

	meta, err := Open(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta.Info.Name != "mydir!" {
		t.Fatalf("name: got %q want %q", meta.Info.Name, "mydir!")
	}
	if !meta.Info.IsMultiFile() {
		t.Fatalf("expected multi-file torrent")
	}
	if len(meta.Info.Files) != 2 {
		t.Fatalf("files len: got %d want %d", len(meta.Info.Files), 2)
	}
	if meta.Info.TotalLength() != 30 {
		t.Fatalf("total length: got %d want %d", meta.Info.TotalLength(), 30)
	}

	if meta.Info.Files[0].Length != 10 {
		t.Fatalf("file 0 length: got %d want %d", meta.Info.Files[0].Length, 10)
	}
	if len(meta.Info.Files[0].Path) != 1 || meta.Info.Files[0].Path[0] != "a.txt" {
		t.Fatalf("file 0 path: got %#v", meta.Info.Files[0].Path)
	}

	if meta.Info.Files[1].Length != 20 {
		t.Fatalf("file 1 length: got %d want %d", meta.Info.Files[1].Length, 20)
	}
	if len(meta.Info.Files[1].Path) != 2 || meta.Info.Files[1].Path[0] != "dir" || meta.Info.Files[1].Path[1] != "b.txt" {
		t.Fatalf("file 1 path: got %#v", meta.Info.Files[1].Path)
	}
}

func TestOpen_InvalidPiecesLength(t *testing.T) {
	input := "" +
		"d" +
		"8:announce" + "14:http://tracker" +
		"4:info" +
		"d" +
		"6:length" + "i123e" +
		"4:name" + "8:file.txt" +
		"12:piece length" + "i16384e" +
		"6:pieces" + "3:abc" +
		"e" +
		"e"

	_, err := Open(strings.NewReader(input))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
