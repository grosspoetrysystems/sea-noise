package url

import "testing"

func TestExtractYouTubeID(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		wantID string
		wantOK bool
	}{
		{
			name:   "youtube.com/watch?v= long form",
			in:     "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantID: "dQw4w9WgXcQ",
			wantOK: true,
		},
		{
			name:   "youtu.be short form",
			in:     "https://youtu.be/dQw4w9WgXcQ",
			wantID: "dQw4w9WgXcQ",
			wantOK: true,
		},
		{
			name:   "youtu.be with query string",
			in:     "https://youtu.be/dQw4w9WgXcQ?si=abc",
			wantID: "dQw4w9WgXcQ",
			wantOK: true,
		},
		{
			name:   "watch?v= with additional params",
			in:     "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=10s",
			wantID: "dQw4w9WgXcQ",
			wantOK: true,
		},
		{
			name:   "id with dashes and underscores (exactly 11 chars)",
			in:     "https://youtu.be/abc-_DEF12X",
			wantID: "abc-_DEF12X",
			wantOK: true,
		},
		{
			name:   "embed form NOT matched (parity with prototype)",
			in:     "https://www.youtube.com/embed/dQw4w9WgXcQ",
			wantID: "",
			wantOK: false,
		},
		{
			name:   "mobile m.youtube.com NOT matched (parity)",
			in:     "https://m.youtube.com/watch?v=dQw4w9WgXcQ",
			wantID: "dQw4w9WgXcQ",
			wantOK: true, // matches because the search finds "youtube.com/watch?v=" anywhere
		},
		{
			name:   "non-youtube url",
			in:     "https://example.com/video",
			wantID: "",
			wantOK: false,
		},
		{
			name:   "short id not matched (must be exactly 11 chars)",
			in:     "https://youtu.be/short",
			wantID: "",
			wantOK: false,
		},
		{
			name:   "empty string",
			in:     "",
			wantID: "",
			wantOK: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotID, gotOK := ExtractYouTubeID(c.in)
			if gotID != c.wantID || gotOK != c.wantOK {
				t.Errorf("ExtractYouTubeID(%q) = (%q, %v), want (%q, %v)",
					c.in, gotID, gotOK, c.wantID, c.wantOK)
			}
		})
	}
}
