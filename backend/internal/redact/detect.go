package redact

import (
	"regexp"
	"strings"
)

const minPhoneDigits = 7

var (
	reEmail      = regexp.MustCompile(`[\w.\-+]+@[\w.\-]+\.\w{2,}`)
	reURL        = regexp.MustCompile(`(?i)https?://[^\s]+|[a-z0-9\-]+\.(com|net|org|io|co|ar|es|mx|br|pt|me|ly|be|app|dev|info|xyz|ws)[^\s]*`)
	reHandle     = regexp.MustCompile(`(?:^|\s)(@[\w.]{3,})`)
	reShortener  = regexp.MustCompile(`(?i)(t\.me|wa\.me|instagram\.com|fb\.com|telegram\.me|wa\.link|bit\.ly|tinyurl\.com)/[^\s]+`)
	rePhone      = regexp.MustCompile(`\+?\d[\d\s\-().]{5,}\d`)
	reNumCluster = regexp.MustCompile(`\d[\d\s\-().]{2,}\d`)
)

var contactKeywords = []string{
	"whatsapp", "wsp", "wpp", "telegram", "insta", "instagram",
	"mi numero", "mi número", "mi tel", "mi cel",
	"escribime a", "escríbime a", "escribeme a", "escríbeme a",
	"escríbeme al", "escribeme al", "escríbime al", "escribime al",
	"pasame el", "pásame el",
	"meu numero", "meu número", "meu tel", "meu cel",
	"escreve pra mim", "escreva pra mim", "me chama no",
	"call me", "text me", "reach me", "my number", "my phone",
	"phone me", "ring me",
	"contáctame", "contactame",
}

// Span is a byte range that covers external contact info.
type Span struct {
	Start int
	End   int
	Kind  string
}

// Detect returns the spans, in ORIGINAL-string byte coordinates, that cover
// external contact info (phones, emails, URLs, social handles). Detection runs
// against the normalized text; spans are translated back to original
// coordinates so the caller can redact in place without mangling the rest of
// the message.
func Detect(original string) []Span {
	norm, srcIdx := normalizeWithMap(original)
	digitNorm := applyHomoglyphsTokenAware(norm)

	var spans []Span
	add := func(start, end int, kind string) {
		if start >= end || alreadyCovered(spans, start, end) {
			return
		}
		spans = append(spans, Span{Start: start, End: end, Kind: kind})
	}

	for _, m := range reShortener.FindAllStringIndex(norm, -1) {
		add(m[0], m[1], "handle")
	}
	for _, m := range reEmail.FindAllStringIndex(norm, -1) {
		add(m[0], m[1], "email")
	}
	for _, m := range reURL.FindAllStringIndex(norm, -1) {
		add(m[0], m[1], "url")
	}
	for _, m := range reHandle.FindAllStringIndex(norm, -1) {
		start, end := m[0], m[1]
		if loc := reHandle.FindStringSubmatchIndex(norm[m[0]:m[1]]); len(loc) >= 4 {
			start = m[0] + loc[2]
			end = m[0] + loc[3]
		}
		add(start, end, "handle")
	}

	for _, m := range rePhone.FindAllStringIndex(digitNorm, -1) {
		matched := digitNorm[m[0]:m[1]]
		if countDigits(matched) < minPhoneDigits && !hasNearbyKeyword(norm, m[0], m[1]) {
			continue
		}
		add(m[0], m[1], "phone")
	}
	for _, m := range reNumCluster.FindAllStringIndex(digitNorm, -1) {
		if !hasNearbyKeyword(norm, m[0], m[1]) {
			continue
		}
		if countDigits(digitNorm[m[0]:m[1]]) < 4 {
			continue
		}
		add(m[0], m[1], "phone")
	}

	out := make([]Span, 0, len(spans))
	for _, s := range spans {
		out = append(out, Span{Start: srcIdx[s.Start], End: srcIdx[s.End], Kind: s.Kind})
	}
	return out
}

func alreadyCovered(spans []Span, start, end int) bool {
	for _, s := range spans {
		if start < s.End && end > s.Start {
			return true
		}
	}
	return false
}

func hasNearbyKeyword(norm string, start, end int) bool {
	const window = 40
	lo := max(start-window, 0)
	hi := min(end+window, len(norm))
	w := norm[lo:hi]
	for _, kw := range contactKeywords {
		if strings.Contains(w, kw) {
			return true
		}
	}
	return false
}

func countDigits(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	return n
}
