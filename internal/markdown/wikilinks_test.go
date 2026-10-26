package markdown

import (
	"reflect"
	"testing"
)

func TestExtractWikilinks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "no links",
			in:   "plain prose with [brackets] and (parens)",
			want: []string{},
		},
		{
			name: "single plain link",
			in:   "see [[Other Note]]",
			want: []string{"Other Note"},
		},
		{
			name: "alias link",
			in:   "see [[Target|Display Text]]",
			want: []string{"Target"},
		},
		{
			name: "section link",
			in:   "see [[Target#Section]]",
			want: []string{"Target"},
		},
		{
			name: "multiple in order",
			in:   "[[a]] then [[b]] then [[c]]",
			want: []string{"a", "b", "c"},
		},
		{
			name: "trims surrounding whitespace",
			in:   "[[  spaced  ]]",
			want: []string{"spaced"},
		},
		{
			name: "ignores triple brackets",
			in:   "[[[x]]]",
			want: []string{"[x"},
		},
		{
			name: "empty target is not matched",
			in:   "[[]]",
			want: []string{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExtractWikilinks(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("ExtractWikilinks(%q) = %#v, want %#v", c.in, got, c.want)
			}
		})
	}
}
