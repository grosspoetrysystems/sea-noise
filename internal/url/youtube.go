package url

import "regexp"

// youtubePattern matches a YouTube video ID embedded in a URL. Mirrors
// references/scripts/fetch-youtube-transcript.py:YT_PATTERN:
//
//	r"(?:youtube\.com/watch\?v=|youtu\.be/)([\w-]{11})"
//
// The pattern uses search semantics (not anchored), so a video ID
// surrounded by other URL components still matches.
var youtubePattern = regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtu\.be/)([\w-]{11})`)

// ExtractYouTubeID returns the 11-character video ID from a YouTube
// URL, or ("", false) if the URL does not contain a recognizable ID.
// Recognizable forms (mirroring the prototype):
//
//   - https://www.youtube.com/watch?v=VIDEOID
//   - https://youtu.be/VIDEOID
//
// Other forms (`/embed/VIDEOID`, mobile `/v/VIDEOID`, etc.) are
// intentionally NOT matched, to preserve parity with the prototype.
// If the prototype adds support later, this function follows.
func ExtractYouTubeID(url string) (string, bool) {
	m := youtubePattern.FindStringSubmatch(url)
	if m == nil {
		return "", false
	}
	return m[1], true
}
