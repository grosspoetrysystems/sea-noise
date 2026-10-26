package url

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain url unchanged",
			in:   "https://example.com/path",
			want: "https://example.com/path",
		},
		{
			name: "trailing slash stripped",
			in:   "https://example.com/path/",
			want: "https://example.com/path",
		},
		{
			name: "fragment stripped",
			in:   "https://example.com/p#section",
			want: "https://example.com/p",
		},
		{
			name: "utm parameters stripped",
			in:   "https://example.com/p?utm_source=x&utm_campaign=y",
			want: "https://example.com/p",
		},
		{
			name: "single utm param leaves trailing ?",
			in:   "https://example.com/p?utm_source=x",
			want: "https://example.com/p",
		},
		{
			// Prototype quirk: when the FIRST query param is a tracking
			// param, stripping it leaves a leading `&`, which the
			// normalize_url cleanup does NOT repair (the leftover-`?&`
			// regex only fires when the cleanup leaves a literal `?&`).
			// We mirror this behavior for parity, even though the result
			// is technically a malformed URL.
			name: "mixed tracking and real params — first-param quirk preserved",
			in:   "https://example.com/p?utm_source=x&q=hello",
			want: "https://example.com/p&q=hello",
		},
		{
			name: "fbclid stripped",
			in:   "https://example.com/p?fbclid=abc",
			want: "https://example.com/p",
		},
		{
			name: "gclid stripped",
			in:   "https://example.com/p?gclid=abc",
			want: "https://example.com/p",
		},
		{
			name: "ref stripped",
			in:   "https://example.com/p?ref=newsletter",
			want: "https://example.com/p",
		},
		{
			name: "mc_* stripped",
			in:   "https://example.com/p?mc_eid=abc&mc_cid=def",
			want: "https://example.com/p",
		},
		{
			name: "whitespace trimmed",
			in:   "  https://example.com/  ",
			want: "https://example.com",
		},
		{
			name: "fragment after tracking",
			in:   "https://example.com/p?utm_source=x#section",
			want: "https://example.com/p",
		},
		{
			// Note: `rstrip('/')` only strips from the END of the string,
			// not from the path segment. The `/` before `?` stays.
			name: "tracking in middle, fragment, slash-only-stripped-at-end",
			in:   "https://example.com/p/?a=1&utm_source=x&b=2#hash",
			want: "https://example.com/p/?a=1&b=2",
		},
		{
			name: "no tracking, no fragment, no trailing slash",
			in:   "https://example.com/foo?q=bar",
			want: "https://example.com/foo?q=bar",
		},
		{
			name: "multiple trailing slashes all stripped",
			in:   "https://example.com/p///",
			want: "https://example.com/p",
		},
		{
			name: "non-tracking 'reference' query not matched as ref",
			in:   "https://example.com/p?reference=ok",
			want: "https://example.com/p?reference=ok",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Normalize(c.in)
			if got != c.want {
				t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
