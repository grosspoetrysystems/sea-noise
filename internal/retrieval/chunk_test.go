package retrieval_test

import (
	"reflect"
	"testing"

	"sea-noise/internal/retrieval"
)

func TestChunkBody(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"whitespace only", "   \n\n  \t", nil},
		{"single paragraph no blanks", "one line\nstill same paragraph", []string{"one line\nstill same paragraph"}},
		{"two paragraphs", "alpha\n\nbeta", []string{"alpha", "beta"}},
		{"crlf blank line", "alpha\r\n\r\nbeta", []string{"alpha", "beta"}},
		{"three paragraphs with blank indented line", "alpha\n  \nbeta\n\n\ngamma", []string{"alpha", "beta", "gamma"}},
		{"trailing blanks", "alpha\n\n\n", []string{"alpha"}},
		{"leading blanks", "\n\nalpha\n\nbeta", []string{"alpha", "beta"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := retrieval.ChunkBody(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ChunkBody(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
