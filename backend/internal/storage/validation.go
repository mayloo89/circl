package storage

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

var (
	ErrInvalidCategory    = errors.New("storage: invalid category")
	ErrInvalidContentType = errors.New("storage: content type not allowed")
	ErrFileTooLarge       = errors.New("storage: file exceeds size limit")
	ErrInvalidFilename    = errors.New("storage: invalid filename")
)

// Category identifies what kind of upload this is.
type Category string

const (
	CategoryAvatar         Category = "avatar"
	CategoryChatAttachment Category = "chat-attachment"
	CategoryProfilePhoto   Category = "profile-photo"
)

// allowedTypes maps each category to its permitted MIME types.
var allowedTypes = map[Category][]string{
	CategoryAvatar: {
		"image/jpeg",
		"image/png",
		"image/webp",
	},
	CategoryChatAttachment: {
		"image/jpeg",
		"image/png",
		"image/webp",
		"image/gif",
		"video/mp4",
		"video/quicktime",
		"application/pdf",
	},
	CategoryProfilePhoto: {
		"image/jpeg",
		"image/png",
		"image/webp",
	},
}

// maxSizes maps each category to its maximum file size in bytes.
var maxSizes = map[Category]int64{
	CategoryAvatar:         5 * 1024 * 1024,  // 5 MB
	CategoryChatAttachment: 50 * 1024 * 1024, // 50 MB
	CategoryProfilePhoto:   10 * 1024 * 1024, // 10 MB
}

// ParseCategory converts a string to a Category, returning an error if invalid.
func ParseCategory(s string) (Category, error) {
	c := Category(s)
	if _, ok := allowedTypes[c]; !ok {
		return "", fmt.Errorf("%w: %q", ErrInvalidCategory, s)
	}
	return c, nil
}

// MaxSize returns the maximum allowed upload size for a category.
func MaxSize(c Category) int64 {
	return maxSizes[c]
}

// ValidateUpload checks content type and size against the rules for a category.
func ValidateUpload(category Category, contentType string, size int64) error {
	types, ok := allowedTypes[category]
	if !ok {
		return fmt.Errorf("%w: %q", ErrInvalidCategory, category)
	}
	if !slices.Contains(types, contentType) {
		return fmt.Errorf("%w: %q for category %q", ErrInvalidContentType, contentType, category)
	}
	limit := maxSizes[category]
	if size > limit {
		return fmt.Errorf("%w: %d bytes exceeds %d byte limit", ErrFileTooLarge, size, limit)
	}
	return nil
}

// SanitizeFilename strips directory components and returns a safe base name.
// Returns an error if the result is empty.
func SanitizeFilename(name string) (string, error) {
	base := filepath.Base(name)
	base = strings.ReplaceAll(base, "..", "")
	base = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == '\x00' {
			return -1
		}
		return r
	}, base)
	base = strings.TrimSpace(base)
	if base == "" || base == "." {
		return "", ErrInvalidFilename
	}
	// Limit length to 255 characters.
	if len(base) > 255 {
		ext := filepath.Ext(base)
		base = base[:255-len(ext)] + ext
	}
	return base, nil
}
