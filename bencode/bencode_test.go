package bencode

import (
	"bytes"
	"reflect"
	"testing"
)

func TestDecode(t *testing.T) {

	tests := []struct {
		name    string
		input   string
		want    Value
		wantErr bool
	}{
		{
			name:  "int zero",
			input: "i0e",
			want:  int64(0),
		},
		{
			name:  "int positive",
			input: "i42e",
			want:  int64(42),
		},
		{
			name:  "int negative",
			input: "i-42e",
			want:  int64(-42),
		},
		{
			name:    "int empty",
			input:   "ie",
			wantErr: true,
		},
		{
			name:    "int leading zero",
			input:   "i03e",
			wantErr: true,
		},
		{
			name:    "int negative zero",
			input:   "i-0e",
			wantErr: true,
		},
		{
			name:  "empty bytes",
			input: "0:",
			want:  []byte(""),
		},
		{
			name:  "bytes",
			input: "4:spam",
			want:  []byte("spam"),
		},
		{
			name:  "empty list",
			input: "le",
			want:  List{},
		},
		{
			name:  "list mixed",
			input: "l4:spami42ee",
			want: List{
				[]byte("spam"),
				int64(42),
			},
		},
		{
			name:  "empty dict",
			input: "de",
			want:  Dict{},
		},
		{
			name:  "dict simple",
			input: "d3:cow3:moo4:spam4:eggse",
			want: Dict{
				"cow":  []byte("moo"),
				"spam": []byte("eggs"),
			},
		},
		{
			name:  "nested",
			input: "d4:listl4:spami1ee3:numi99ee",
			want: Dict{
				"list": List{
					[]byte("spam"),
					int64(1),
				},
				"num": int64(99),
			},
		},
		{
			name:    "trailing junk",
			input:   "i42ei43e",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewDecoder(bytes.NewReader([]byte(tt.input)))
			got, err := dec.Decode()

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !deepEqualValue(got, tt.want) {
				t.Fatalf("got %#v (%T), want %#v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func deepEqualValue(a, b any) bool {
	return reflect.DeepEqual(a, b)
}
