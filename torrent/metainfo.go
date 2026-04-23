package torrent

import (
	"io"

	"github.com/elliota43/gotorrent/bencode"
)

type MetaInfo struct {
	Pieces      string `bencode:"pieces"`
	PieceLength int    `bencode:"piece length"`
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
}

type TorrentMeta struct {
	Announce string   `bencode:"announce"`
	Info     MetaInfo `bencode:"info"`
}

func Open(r io.Reader) (*TorrentMeta, error) {
	var meta TorrentMeta
	if err := bencode.Unmarshal(r, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}
