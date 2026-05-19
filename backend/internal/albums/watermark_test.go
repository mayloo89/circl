package albums

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
	"time"
)

func makeTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makeTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCompositeWatermark_JPEG(t *testing.T) {
	src := makeTestJPEG(t, 400, 300)
	ts := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	out, ct, err := compositeWatermark(bytes.NewReader(src), "image/jpeg", "alice", ts)
	if err != nil {
		t.Fatalf("composite: %v", err)
	}
	if ct != "image/jpeg" {
		t.Errorf("content-type = %q, want image/jpeg", ct)
	}
	// Output must decode as a valid JPEG.
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("output not valid JPEG: %v", err)
	}
}

func TestCompositeWatermark_PNG(t *testing.T) {
	src := makeTestPNG(t, 200, 200)
	ts := time.Now()
	out, ct, err := compositeWatermark(bytes.NewReader(src), "image/png", "bob", ts)
	if err != nil {
		t.Fatalf("composite: %v", err)
	}
	if ct != "image/png" {
		t.Errorf("content-type = %q, want image/png", ct)
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("output not valid PNG: %v", err)
	}
}

func TestCompositeWatermark_UnknownTypePassthrough(t *testing.T) {
	payload := []byte("not an image")
	out, ct, err := compositeWatermark(bytes.NewReader(payload), "application/octet-stream", "x", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ct != "application/octet-stream" {
		t.Errorf("content-type = %q, want application/octet-stream", ct)
	}
	if !bytes.Equal(out, payload) {
		t.Error("payload was modified for unknown content type")
	}
}

func TestCompositeWatermark_LabelContainsName(t *testing.T) {
	// Indirect check: compositing the same image twice with different names
	// must produce different byte sequences.
	src := makeTestJPEG(t, 300, 200)
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	out1, _, _ := compositeWatermark(bytes.NewReader(src), "image/jpeg", "alice", ts)
	out2, _, _ := compositeWatermark(bytes.NewReader(src), "image/jpeg", "bob", ts)
	if bytes.Equal(out1, out2) {
		t.Error("expected different outputs for different watermark names")
	}
}

func TestCompositeWatermark_WebPReencodesAsJPEG(t *testing.T) {
	// We can't easily produce a real WebP in a unit test without a encoder,
	// so verify that passing a corrupted webp body returns an error (not a
	// silent passthrough), which confirms the webp branch is entered.
	_, _, err := compositeWatermark(strings.NewReader("not webp"), "image/webp", "x", time.Now())
	if err == nil {
		t.Error("expected decode error for invalid webp input")
	}
}
