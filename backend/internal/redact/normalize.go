package redact

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// replacement is a detection-only normalization rule. Replacement values are
// never shown to users — Redact emits text from the original string — so these
// rules may be lossy. They exist purely to make detection regexes match
// obfuscated contact info.
type replacement struct {
	key []rune
	val string
}

var replacements = buildReplacements()

func buildReplacements() []replacement {
	raw := map[string]string{
		"punto com": ".com", "ponto com": ".com", "dot com": ".com",
		"arrobado": "@", "arroba": "@", "aroba": "@",
		"punto": ".", "ponto": ".", "at": "@",
		// spelled-out digits — Spanish
		"uno": "1", "dos": "2", "tres": "3", "cuatro": "4", "cinco": "5",
		"seis": "6", "siete": "7", "ocho": "8", "nueve": "9",
		// Portuguese
		"um": "1", "dois": "2", "três": "3", "quatro": "4", "sete": "7",
		"oito": "8", "nove": "9",
		// English
		"zero": "0", "one": "1", "two": "2", "three": "3", "four": "4",
		"five": "5", "six": "6", "seven": "7", "eight": "8", "nine": "9",
	}
	out := make([]replacement, 0, len(raw))
	for k, v := range raw {
		out = append(out, replacement{key: []rune(k), val: v})
	}
	// Longest key first so multi-word phrases ("punto com") win over their
	// prefixes ("punto").
	sort.Slice(out, func(i, j int) bool { return len(out[i].key) > len(out[j].key) })
	return out
}

// Normalize lowercases the input and applies the detection-only replacement
// rules. The result is used only for matching, never shown to users.
func Normalize(in string) string {
	s, _ := normalizeWithMap(in)
	return s
}

// normalizeWithMap returns the normalized string and a per-byte index mapping
// each normalized byte back to its originating byte offset in the original
// string. srcIdx has len(norm)+1 entries; the final entry is len(original) so
// an exclusive span end can be translated. This is what lets Redact replace
// only the detected spans while emitting the rest of the original verbatim.
func normalizeWithMap(original string) (string, []int) {
	runes := []rune(original)
	offs := make([]int, len(runes)+1)
	b := 0
	for i, r := range runes {
		offs[i] = b
		b += utf8.RuneLen(r)
	}
	offs[len(runes)] = b

	var norm strings.Builder
	srcIdx := make([]int, 0, len(original)+1)
	emit := func(s string, src int) {
		norm.WriteString(s)
		for range len(s) {
			srcIdx = append(srcIdx, src)
		}
	}

	i := 0
	for i < len(runes) {
		if val, n, ok := matchReplacement(runes, i); ok {
			emit(val, offs[i])
			i += n
			continue
		}
		emit(string(unicode.ToLower(runes[i])), offs[i])
		i++
	}
	srcIdx = append(srcIdx, offs[len(runes)])
	return norm.String(), srcIdx
}

// matchReplacement reports whether a replacement rule matches at rune index i,
// respecting word boundaries (no letter immediately before or after) so that
// "uno" does not match inside "alguno".
func matchReplacement(runes []rune, i int) (string, int, bool) {
	if i > 0 && unicode.IsLetter(runes[i-1]) {
		return "", 0, false
	}
	for _, rp := range replacements {
		n := len(rp.key)
		if i+n > len(runes) {
			continue
		}
		match := true
		for k := range n {
			if unicode.ToLower(runes[i+k]) != rp.key[k] {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		if i+n < len(runes) && unicode.IsLetter(runes[i+n]) {
			continue
		}
		return rp.val, n, true
	}
	return "", 0, false
}

// NormalizeForDigits applies token-aware homoglyph substitution on top of
// Normalize, used only for phone detection. Homoglyphs (o->0, l->1, ...) are
// converted only inside whitespace-delimited tokens that already contain a
// real digit, so ordinary words like "looool" or "ooooh" are never turned into
// digit runs. Substitution is byte-length preserving, so it keeps the offset
// mapping from normalizeWithMap valid.
func NormalizeForDigits(in string) string {
	return applyHomoglyphsTokenAware(Normalize(in))
}

func applyHomoglyphsTokenAware(s string) string {
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(runes) {
		if unicode.IsSpace(runes[i]) {
			b.WriteRune(runes[i])
			i++
			continue
		}
		j := i
		hasDigit := false
		for j < len(runes) && !unicode.IsSpace(runes[j]) {
			if runes[j] >= '0' && runes[j] <= '9' {
				hasDigit = true
			}
			j++
		}
		for _, r := range runes[i:j] {
			b.WriteRune(homoglyph(r, hasDigit))
		}
		i = j
	}
	return b.String()
}

func homoglyph(r rune, active bool) rune {
	if !active {
		return r
	}
	switch r {
	case 'o':
		return '0'
	case 'l', 'i':
		return '1'
	case 's':
		return '5'
	case 'b':
		return '8'
	default:
		return r
	}
}
