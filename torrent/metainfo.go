package torrent

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"

	"github.com/elliota43/gotorrent/bencode"
)

type TorrentMeta struct {
	Announce     string         `bencode:"announce"`
	AnnounceList [][]string     `bencode:"announce-list"`
	Comment      string         `bencode:"comment"`
	CreatedBy    string         `bencode:"created by"`
	CreationDate int64          `bencode:"creation date"`
	Encoding     string         `bencode:"encoding"`
	Info         InfoDictionary `bencode:"info"`
	InfoBytes    []byte         `bencode:"info,raw"`
	InfoHash     [20]byte       `bencode:"info,sha1"`
}

type InfoDictionary struct {
	PieceLength int        `bencode:"piece length"`
	Pieces      []byte     `bencode:"pieces"`
	Private     int64      `bencode:"private"` // 1 means private when present (rare)
	Name        string     `bencode:"name"`
	Length      int64      `bencode:"length"`
	Files       []FileInfo `bencode:"files"`
}

type FileInfo struct {
	Length int64    `bencode:"length"`
	Path   []string `bencode:"path"`
}

func Open(r io.Reader) (*TorrentMeta, error) {
	var meta TorrentMeta
	if err := bencode.Unmarshal(r, &meta); err != nil {
		return nil, err
	}
	if err := meta.Info.Validate(); err != nil {
		return nil, err
	}
	return &meta, nil
}

func (i InfoDictionary) IsMultiFile() bool {
	return len(i.Files) > 0
}

func (i InfoDictionary) IsSingleFile() bool {
	// single file torrents have no Files field
	return len(i.Files) == 0
}

func (i InfoDictionary) TotalLength() int64 {
	if i.IsSingleFile() {
		return i.Length
	}

	var total int64
	for _, f := range i.Files {
		total += f.Length
	}

	return total
}

func (i InfoDictionary) LastPieceLength() int64 {
	last := i.TotalLength() % int64(i.PieceLength)
	if last == 0 {
		last = int64(i.PieceLength)
	}

	return last
}

func (i InfoDictionary) PieceCount() int {
	return len(i.Pieces) / sha1.Size
}

func (i InfoDictionary) Validate() error {
	if i.PieceLength <= 0 {
		return errors.New("torrent: piece length must be > 0")
	}

	if len(i.Pieces)%sha1.Size != 0 {
		return fmt.Errorf("torrent: pieces length %d is not a multiple of %d", len(i.Pieces), sha1.Size)
	}

	if i.IsSingleFile() {
		if i.Length <= 0 {
			return errors.New("torrent: single-file length must be > 0")
		}
		return nil
	}

	for idx, f := range i.Files {
		if f.Length < 0 {
			return fmt.Errorf("torrent: file %d has negative length", idx)
		}
		if len(f.Path) == 0 {
			return fmt.Errorf("torrent: file %d has empty path", idx)
		}
	}

	return nil
}

func (i InfoDictionary) PieceHashes() ([][sha1.Size]byte, error) {
	if len(i.Pieces)%sha1.Size != 0 {
		return nil, fmt.Errorf("torrent: pieces length %d is not a multiple of %d", len(i.Pieces), sha1.Size)
	}

	count := len(i.Pieces) / sha1.Size
	out := make([][sha1.Size]byte, count)

	for idx := 0; idx < count; idx++ {
		start := idx * sha1.Size
		end := start + sha1.Size
		copy(out[idx][:], i.Pieces[start:end])
	}

	return out, nil
}
