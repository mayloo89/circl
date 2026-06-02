package redact_test

import (
	"strings"
	"testing"

	"github.com/mayloo89/circl/backend/internal/redact"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercases", "Hello World", "hello world"},
		{"arroba replacement", "mandame arroba gmail", "mandame @ gmail"},
		{"punto com replacement", "mandame punto com", "mandame .com"},
		{"spelled digit uno", "mi numero uno dos tres", "mi numero 1 2 3"},
		{"spelled digit one", "call me at one two three", "call me @ 1 2 3"},
		{"spelled digit Portuguese um", "meu numero um dois", "meu numero 1 2"},
		{"dot com in URL pattern", "google punto com", "google .com"},
		{"empty string", "", ""},
		{"no changes needed", "hola como estas", "hola como estas"},
		{"at sign word", "contact me at gmail", "contact me @ gmail"},
		{"punto before com", "gmail punto com", "gmail .com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redact.Normalize(tt.input)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeForDigits(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"homoglyph only in digit token", "call O1234", "call 01234"},
		{"homoglyph l->1", "l2345", "12345"},
		{"spelled digits, no homoglyph on word", "mi numero uno dos tres", "mi numero 1 2 3"},
		{"interjection left untouched", "ooooooo looooool", "ooooooo looooool"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redact.NormalizeForDigits(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeForDigits(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHit  bool
		wantKind string
	}{
		{"phone AR mobile", "mi numero es +549111234567", true, "phone"},
		{"phone 8 digits", "llamame al 12345678", true, "phone"},
		{"phone with dashes", "call 123-456-7890", true, "phone"},
		{"email plain", "escribime a user@gmail.com", true, "email"},
		{"email no keyword", "user@gmail.com", true, "email"},
		{"url https", "visita https://example.com", true, "url"},
		{"url t.me", "mi telegram es t.me/user123", true, "handle"},
		{"wa.me link", "hablame por wa.me/549111234", true, "handle"},
		{"@handle", "mi user es @johndoe", true, "handle"},
		{"keyword mi numero + short digits", "mi numero es 1234", true, "phone"},
		{"keyword escribime + short number", "escribime al 1234", true, "phone"},
		{"keyword pasame el", "pasame el numero 15-5555-1234", true, "phone"},
		{"keyword call me", "call me at 5551234", true, "phone"},
		{"keyword me chama no", "me chama no 11987654321", true, "phone"},
		{"email with plus", "write to me+tag@domain.co", true, "email"},
		{"telegram handle", "mi telegram @user123", true, "handle"},
		{"wsp keyword + phone", "hablame al wsp 1551234567", true, "phone"},
		{"mixed safe and phone", "hola! mi numero es 1551234567 nos vemos", true, "phone"},
		{"phone with parens", "llama al (11) 4567-8901", true, "phone"},
		{"false positive age", "tengo 28", false, ""},
		{"false positive height", "mido 1.80", false, ""},
		{"false positive time", "nos vemos a las 15:30", false, ""},
		{"false positive price", "cuesta 2500 pesos", false, ""},
		{"safe text", "hola como estas?", false, ""},
		{"safe text es", "buen dia, todo bien?", false, ""},
		{"safe text pt", "bom dia, tudo bem?", false, ""},
		{"false positive year", "en el 2024", false, ""},
		{"false positive age tengo", "tengo 30 años", false, ""},
		{"interjection ooooh not a phone", "jaja siii ooooooo", false, ""},
		{"interjection looool not a phone", "nooo looooool que risa", false, ""},
		{"long laugh not a phone", "bahhh bbbbbbbb", false, ""},
		{"url shortener bit.ly", "mira bit.ly/abc123", true, "handle"},
		{"url .app domain", "visita myapp.app/page", true, "url"},
		{"instagram.com handle", "seguime en instagram.com/user", true, "handle"},
		{"whatsapp keyword", "hablame por whatsapp al 11-2345-6789", true, "phone"},
		{"phone with spaces", "mi tel es 11 2345 6789", true, "phone"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spans := redact.Detect(tt.input)
			if tt.wantHit {
				if len(spans) == 0 {
					t.Errorf("Detect(%q): expected hit, got none", tt.input)
					return
				}
				found := false
				for _, s := range spans {
					if s.Kind == tt.wantKind {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Detect(%q): expected kind %q, got %v", tt.input, tt.wantKind, spans)
				}
			} else {
				if len(spans) > 0 {
					t.Errorf("Detect(%q): expected no hits, got %v", tt.input, spans)
				}
			}
		})
	}
}

func TestRedact(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantToken bool
		wantSafe  bool
	}{
		{"phone replaced", "llamame al 12345678 despues", true, true},
		{"email replaced", "escribime a user@gmail.com porfa", true, true},
		{"url replaced", "mira https://example.com ok", true, true},
		{"handle replaced", "mi user @johndoke chateamos", true, true},
		{"safe text untouched", "hola como estas?", false, false},
		{"all contact replaced", "mi numero es +549111234567", true, false},
		{"multiple hits", "mi email a@b.com y tel 12345678", true, true},
		{"empty string", "", false, false},
		{"age not replaced", "tengo 28", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := redact.Redact(tt.input)
			if res.Redacted != tt.wantToken {
				t.Errorf("Redact(%q).Redacted = %v, want %v", tt.input, res.Redacted, tt.wantToken)
			}
			if tt.wantToken && !strings.Contains(res.Content, redact.RedactionToken) {
				t.Errorf("Redact(%q).Content = %q, expected to contain %q", tt.input, res.Content, redact.RedactionToken)
			}
			if tt.wantSafe && res.Content == redact.RedactionToken {
				t.Errorf("Redact(%q).Content = %q, expected partial redaction (surrounding text preserved)", tt.input, res.Content)
			}
			if !tt.wantToken && res.Content != tt.input {
				t.Errorf("Redact(%q).Content = %q, expected unchanged", tt.input, res.Content)
			}
		})
	}
}

// TestRedactPreservesSurroundingText guards the bug where the whole message was
// rebuilt from the normalized string, lowercasing legit text and turning
// spelled-out words ("dos" -> "2") into digits. Only the detected span must be
// replaced; the rest must survive verbatim.
func TestRedactPreservesSurroundingText(t *testing.T) {
	tests := []struct {
		input          string
		wantContains   []string
		wantNotContain []string
	}{
		{
			input:          "Llamame Hoy, mi numero es 12345678",
			wantContains:   []string{"Llamame Hoy,", redact.RedactionToken},
			wantNotContain: []string{"llamame hoy", "12345678"},
		},
		{
			input:          "Te debo dos cafes, llamame al 12345678",
			wantContains:   []string{"Te debo dos cafes,", redact.RedactionToken},
			wantNotContain: []string{"2 cafes", "12345678"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := redact.Redact(tt.input).Content
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("Redact(%q).Content = %q, expected to contain %q", tt.input, got, want)
				}
			}
			for _, bad := range tt.wantNotContain {
				if strings.Contains(got, bad) {
					t.Errorf("Redact(%q).Content = %q, expected NOT to contain %q", tt.input, got, bad)
				}
			}
		})
	}
}
