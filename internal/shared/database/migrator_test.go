package database

import (
	"bytes"
	"testing"
)

func TestStripBOM(t *testing.T) {
	tests := []struct {
		name string
		give []byte
		want []byte
	}{
		{
			name: "With UTF-8 BOM",
			give: append([]byte{0xEF, 0xBB, 0xBF}, []byte("SELECT 1;")...),
			want: []byte("SELECT 1;"),
		},
		{
			name: "Without BOM",
			give: []byte("SELECT 1;"),
			want: []byte("SELECT 1;"),
		},
		{
			name: "Empty slice",
			give: []byte{},
			want: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bytes.TrimPrefix(tt.give, []byte("\xef\xbb\xbf"))
			if !bytes.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
