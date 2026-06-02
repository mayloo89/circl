package redact

import (
	"sort"
	"strings"
)

// RedactionToken is the sentinel that replaces a detected contact span in the
// stored content. The frontend swaps it for a localized label at render time.
const RedactionToken = "[contact hidden]"

// Result is the outcome of redacting a message.
type Result struct {
	Content  string
	Redacted bool
}

// Redact replaces external contact info in original with RedactionToken,
// emitting the rest of the original text verbatim (case and surrounding
// content preserved). The raw contact substring is never carried into the
// result.
func Redact(original string) Result {
	merged := mergeSpans(Detect(original))
	if len(merged) == 0 {
		return Result{Content: original, Redacted: false}
	}

	var b strings.Builder
	prev := 0
	for _, s := range merged {
		if s.Start > prev {
			b.WriteString(original[prev:s.Start])
		}
		b.WriteString(RedactionToken)
		prev = s.End
	}
	if prev < len(original) {
		b.WriteString(original[prev:])
	}
	return Result{Content: b.String(), Redacted: true}
}

// mergeSpans sorts spans by start offset and coalesces any that overlap or
// touch, so adjacent detections produce a single redaction token.
func mergeSpans(spans []Span) []Span {
	if len(spans) == 0 {
		return nil
	}
	sorted := make([]Span, len(spans))
	copy(sorted, spans)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })

	merged := []Span{sorted[0]}
	for _, s := range sorted[1:] {
		last := &merged[len(merged)-1]
		if s.Start <= last.End {
			if s.End > last.End {
				last.End = s.End
			}
			continue
		}
		merged = append(merged, s)
	}
	return merged
}
