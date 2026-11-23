package storage

import (
	"path"
	"strings"
)

// Key prefixes for objects that must never be served by URL. They are read
// only through authenticated code paths (GetObject), never through PublicURL.
const (
	// QuarantinePrefix holds CSAM-class moderation hits preserved for reporting.
	QuarantinePrefix = "quarantine/"
	// EvidencePrefix holds copies of reported media kept with their report.
	EvidencePrefix = "evidence/"
	// ExportPrefix holds data-export archives, downloaded via a token handler.
	ExportPrefix = "exports/"
)

var restrictedPrefixes = []string{QuarantinePrefix, EvidencePrefix, ExportPrefix}

// IsRestrictedKey reports whether key falls under a never-served prefix. The
// key is normalised first, and compared case-insensitively because a
// case-insensitive filesystem would map "Evidence/x" onto "evidence/x".
func IsRestrictedKey(key string) bool {
	clean := strings.ToLower(strings.TrimLeft(path.Clean("/"+key), "/"))
	for _, p := range restrictedPrefixes {
		if clean+"/" == p || strings.HasPrefix(clean, p) {
			return true
		}
	}
	return false
}
