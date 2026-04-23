package main

import (
	"fmt"
	"log"
	"os"

	"github.com/elliota43/gotorrent/torrent"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s <file.torrent>", os.Args[0])
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	meta, err := torrent.Open(f)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Torrent Metadata ===")
	fmt.Printf("Announce: %s\n", meta.Announce)
	fmt.Printf("Name: %s\n", meta.Info.Name)
	fmt.Printf("Piece Length: %d\n", meta.Info.PieceLength)
	fmt.Printf("Length: %d\n", meta.Info.Length)
	fmt.Printf("Pieces raw length: %d\n", len(meta.Info.Pieces))
}
