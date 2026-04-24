package peer

type Bitfield []byte

func (b Bitfield) HasPiece(index int) bool {
	if index < 0 {
		return false
	}

	byteIndex := index / 8
	bitOffset := index % 8

	if byteIndex >= len(b) {
		return false
	}

	return b[byteIndex]&(1<<(7-bitOffset)) != 0
}

func (b Bitfield) SetPiece(index int) {
	if index < 0 {
		return
	}

	byteIndex := index / 8
	bitOffset := index % 8

	if byteIndex >= len(b) {
		return
	}

	b[byteIndex] |= 1 << (7 - bitOffset)
}
