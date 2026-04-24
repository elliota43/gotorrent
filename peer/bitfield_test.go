package peer

import "testing"

func TestBitfieldHasPiece(t *testing.T) {
	tests := []struct {
		name       string
		bitfield   Bitfield
		pieceIndex int
		want       bool
	}{
		{
			name:       "piece 0 set",
			bitfield:   Bitfield{0b10000000},
			pieceIndex: 0,
			want:       true,
		},
		{
			name:       "piece 1 set",
			bitfield:   Bitfield{0b01000000},
			pieceIndex: 1,
			want:       true,
		},
		{
			name:       "piece 7 set",
			bitfield:   Bitfield{0b00000001},
			pieceIndex: 7,
			want:       true,
		},
		{
			name:       "piece 8 set in second byte",
			bitfield:   Bitfield{0b00000000, 0b10000000},
			pieceIndex: 8,
			want:       true,
		},
		{
			name:       "piece not set",
			bitfield:   Bitfield{0b10000000},
			pieceIndex: 1,
			want:       false,
		},
		{
			name:       "piece index out of range",
			bitfield:   Bitfield{0b11111111},
			pieceIndex: 8,
			want:       false,
		},
		{
			name:       "negative piece index",
			bitfield:   Bitfield{0b11111111},
			pieceIndex: -1,
			want:       false,
		},
		{
			name:       "empty bitfield",
			bitfield:   Bitfield{},
			pieceIndex: 0,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bitfield.HasPiece(tt.pieceIndex)
			if got != tt.want {
				t.Fatalf("HasPiece(%d) = %v, want %v", tt.pieceIndex, got, tt.want)
			}
		})
	}
}

func TestBitfieldSetPiece(t *testing.T) {
	tests := []struct {
		name       string
		bitfield   Bitfield
		pieceIndex int
		want       Bitfield
	}{
		{
			name:       "set piece 0",
			bitfield:   Bitfield{0b00000000},
			pieceIndex: 0,
			want:       Bitfield{0b10000000},
		},
		{
			name:       "set piece 1",
			bitfield:   Bitfield{0b00000000},
			pieceIndex: 1,
			want:       Bitfield{0b01000000},
		},
		{
			name:       "set piece 7",
			bitfield:   Bitfield{0b00000000},
			pieceIndex: 7,
			want:       Bitfield{0b00000001},
		},
		{
			name:       "set piece 8",
			bitfield:   Bitfield{0b00000000, 0b00000000},
			pieceIndex: 8,
			want:       Bitfield{0b00000000, 0b10000000},
		},
		{
			name:       "set piece preserves existing bits",
			bitfield:   Bitfield{0b10000000},
			pieceIndex: 1,
			want:       Bitfield{0b11000000},
		},
		{
			name:       "negative index does nothing",
			bitfield:   Bitfield{0b00000000},
			pieceIndex: -1,
			want:       Bitfield{0b00000000},
		},
		{
			name:       "out of range does nothing",
			bitfield:   Bitfield{0b00000000},
			pieceIndex: 8,
			want:       Bitfield{0b00000000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.bitfield.SetPiece(tt.pieceIndex)

			if len(tt.bitfield) != len(tt.want) {
				t.Fatalf("len(bitfield) = %d, want %d", len(tt.bitfield), len(tt.want))
			}

			for i := range tt.want {
				if tt.bitfield[i] != tt.want[i] {
					t.Fatalf("bitfield[%d] = %08b, want %08b", i, tt.bitfield[i], tt.want[i])
				}
			}
		})
	}
}
