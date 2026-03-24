package storage

import (
	"errors"
	"testing"
)

func TestParseCategory(t *testing.T) {
	tests := []struct {
		input string
		want  Category
		err   error
	}{
		{"avatar", CategoryAvatar, nil},
		{"chat-attachment", CategoryChatAttachment, nil},
		{"profile-photo", CategoryProfilePhoto, nil},
		{"invalid", "", ErrInvalidCategory},
		{"", "", ErrInvalidCategory},
	}
	for _, tt := range tests {
		got, err := ParseCategory(tt.input)
		if !errors.Is(err, tt.err) {
			t.Errorf("ParseCategory(%q) error = %v, want %v", tt.input, err, tt.err)
		}
		if got != tt.want {
			t.Errorf("ParseCategory(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidateUpload(t *testing.T) {
	tests := []struct {
		name        string
		category    Category
		contentType string
		size        int64
		wantErr     error
	}{
		{"valid avatar jpeg", CategoryAvatar, "image/jpeg", 1024, nil},
		{"valid avatar png", CategoryAvatar, "image/png", 1024, nil},
		{"valid avatar webp", CategoryAvatar, "image/webp", 1024, nil},
		{"avatar gif rejected", CategoryAvatar, "image/gif", 1024, ErrInvalidContentType},
		{"avatar too large", CategoryAvatar, "image/jpeg", 6 * 1024 * 1024, ErrFileTooLarge},
		{"valid chat jpeg", CategoryChatAttachment, "image/jpeg", 1024, nil},
		{"valid chat pdf", CategoryChatAttachment, "application/pdf", 1024, nil},
		{"valid chat mp4", CategoryChatAttachment, "video/mp4", 1024, nil},
		{"chat too large", CategoryChatAttachment, "image/jpeg", 51 * 1024 * 1024, ErrFileTooLarge},
		{"chat exe rejected", CategoryChatAttachment, "application/x-msdownload", 1024, ErrInvalidContentType},
		{"valid profile photo jpeg", CategoryProfilePhoto, "image/jpeg", 1024, nil},
		{"valid profile photo webp", CategoryProfilePhoto, "image/webp", 1024, nil},
		{"profile photo gif rejected", CategoryProfilePhoto, "image/gif", 1024, ErrInvalidContentType},
		{"profile photo too large", CategoryProfilePhoto, "image/jpeg", 11 * 1024 * 1024, ErrFileTooLarge},
		{"invalid category", Category("bogus"), "image/jpeg", 1024, ErrInvalidCategory},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUpload(tt.category, tt.contentType, tt.size)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateUpload() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"photo.jpg", "photo.jpg", false},
		{"../../../etc/passwd", "passwd", false},
		{"path/to/file.png", "file.png", false},
		{"file\x00name.jpg", "filename.jpg", false},
		{"", "", true},
		{".", "", true},
		{"..", "", true},
		{"   ", "", true},
	}
	for _, tt := range tests {
		got, err := SanitizeFilename(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("SanitizeFilename(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMaxSize(t *testing.T) {
	if got := MaxSize(CategoryAvatar); got != 5*1024*1024 {
		t.Errorf("MaxSize(avatar) = %d, want %d", got, 5*1024*1024)
	}
	if got := MaxSize(CategoryChatAttachment); got != 50*1024*1024 {
		t.Errorf("MaxSize(chat-attachment) = %d, want %d", got, 50*1024*1024)
	}
	if got := MaxSize(CategoryProfilePhoto); got != 10*1024*1024 {
		t.Errorf("MaxSize(profile-photo) = %d, want %d", got, 10*1024*1024)
	}
}
