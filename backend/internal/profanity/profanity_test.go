package profanity_test

import (
	"testing"

	"github.com/mayloo89/circl/backend/internal/profanity"
)

func TestCheck(t *testing.T) {
	clean := []string{
		"Alice", "bob123", "Gamer99", "Hello World",
		"María", "Pedro", "João",
	}
	for _, nick := range clean {
		if profanity.Check(nick) {
			t.Errorf("Check(%q) = true, want false (clean name flagged)", nick)
		}
	}

	dirty := []string{
		"nigger",        // exact
		"n1gg3r",        // leet substitution
		"N1GG3R",        // uppercase + leet
		"puta123",       // Spanish profanity with suffix
		"super_faggot",  // English profanity embedded
		"maric0n",       // Spanish leet
		"buceta",        // Portuguese
	}
	for _, nick := range dirty {
		if !profanity.Check(nick) {
			t.Errorf("Check(%q) = false, want true (profanity not caught)", nick)
		}
	}
}
