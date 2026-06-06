// Package profanity provides a lightweight nickname filter for guest entries.
// It normalises leet-speak substitutions before checking against a deny list
// so that obvious evasion attempts (e.g. "n1gger", "@sshole") are caught.
package profanity

import (
	"strings"
	"unicode"
)

// replacer maps common leet-speak and lookalike characters to their base form.
var replacer = strings.NewReplacer(
	"0", "o",
	"1", "i",
	"3", "e",
	"4", "a",
	"5", "s",
	"@", "a",
	"$", "s",
	"+", "t",
	"!", "i",
)

// normalize lowercases s, strips non-alphanumeric characters (except spaces),
// and applies leet-speak replacements so that "n1gg3r" → "nigger".
func normalize(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '@' || r == '$' || r == '+' || r == '!' {
			b.WriteRune(r)
		}
	}
	return replacer.Replace(b.String())
}

// Check returns true when the nickname contains a word from the deny list.
// Both the full string and individual space-separated tokens are checked so
// that "bad_word_here" (with stripping) is caught alongside "bad word".
func Check(nickname string) bool {
	norm := normalize(nickname)
	for _, word := range denyList {
		if strings.Contains(norm, word) {
			return true
		}
	}
	return false
}
