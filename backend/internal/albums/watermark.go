package albums

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	stdDraw "image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"time"

	xDraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/webp"
)

// wmScale is the SSAA factor: the pill is rendered wmScale× larger and then
// scaled back down with CatmullRom, giving smooth glyphs and pill edges at the
// original target size.
const wmScale = 3

var wmHiResFace font.Face

func init() {
	tt, err := opentype.Parse(goregular.TTF)
	if err != nil {
		panic("albums: parse goregular: " + err.Error())
	}
	// Render at wmScale × DPI so the downscale produces the right final size.
	wmHiResFace, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    11,
		DPI:     96 * wmScale,
		Hinting: font.HintingFull,
	})
	if err != nil {
		panic("albums: new watermark face: " + err.Error())
	}
}

// compositeWatermark overlays a semi-transparent pill label
// ("<uid[:8]> · MM-DD HH:MM") at the bottom-right of the image.
// WebP input is decoded and re-encoded as JPEG (WebP encoding requires cgo).
// Unrecognised content types are returned as-is without modification.
func compositeWatermark(src io.Reader, contentType, uid string, t time.Time) ([]byte, string, error) {
	var (
		img     image.Image
		err     error
		outType = contentType
	)
	switch contentType {
	case "image/jpeg":
		img, err = jpeg.Decode(src)
	case "image/png":
		img, err = png.Decode(src)
	case "image/webp":
		img, err = webp.Decode(src)
		outType = "image/jpeg"
	default:
		data, readErr := io.ReadAll(src)
		if readErr != nil {
			return nil, contentType, readErr
		}
		return data, contentType, nil
	}
	if err != nil {
		return nil, outType, fmt.Errorf("watermark decode: %w", err)
	}

	bounds := img.Bounds()
	out := image.NewRGBA(bounds)
	stdDraw.Draw(out, bounds, img, bounds.Min, stdDraw.Src)

	shortUID := uid
	if len(shortUID) > 12 {
		shortUID = shortUID[:12]
	}
	label := shortUID + " · " + t.UTC().Format("2006-01-02 15:04 UTC")
	drawWMPill(out, label, bounds.Dx(), bounds.Dy())

	var buf bytes.Buffer
	switch outType {
	case "image/jpeg":
		err = jpeg.Encode(&buf, out, &jpeg.Options{Quality: 92})
	case "image/png":
		err = png.Encode(&buf, out)
	}
	if err != nil {
		return nil, outType, fmt.Errorf("watermark encode: %w", err)
	}
	return buf.Bytes(), outType, nil
}

const (
	wmPadH   = 8  // horizontal padding inside pill (target px)
	wmPadV   = 4  // vertical padding inside pill (target px)
	wmRadius = 4  // corner radius (target px)
	wmMargin = 10 // distance from image edge (target px)
)

func drawWMPill(img *image.RGBA, label string, w, h int) {
	s := wmScale
	metrics := wmHiResFace.Metrics()

	// Measure in hi-res pixels.
	d := &font.Drawer{Face: wmHiResFace}
	textW := int(d.MeasureString(label) >> 6)
	textH := int((metrics.Ascent+metrics.Descent)>>6) + 1
	pillW := textW + 2*wmPadH*s
	pillH := textH + 2*wmPadV*s

	// Render pill + text onto a transparent hi-res buffer.
	hiRes := image.NewRGBA(image.Rect(0, 0, pillW, pillH))
	fillRoundedRect(hiRes, 0, 0, pillW-1, pillH-1, wmRadius*s, color.RGBA{0, 0, 0, 140})
	d = &font.Drawer{
		Dst:  hiRes,
		Src:  image.NewUniform(color.RGBA{255, 255, 255, 230}),
		Face: wmHiResFace,
		Dot: fixed.Point26_6{
			X: fixed.I(wmPadH * s),
			Y: fixed.I(wmPadV*s) + metrics.Ascent,
		},
	}
	d.DrawString(label)

	// Scale down to target size with CatmullRom for smooth antialiasing.
	tW, tH := pillW/s, pillH/s
	scaled := image.NewRGBA(image.Rect(0, 0, tW, tH))
	xDraw.CatmullRom.Scale(scaled, scaled.Bounds(), hiRes, hiRes.Bounds(), xDraw.Src, nil)

	// Composite onto main image at bottom-right.
	x1 := w - wmMargin
	y1 := h - wmMargin
	stdDraw.Draw(img, image.Rect(x1-tW, y1-tH, x1, y1), scaled, image.Point{}, stdDraw.Over)
}

// fillRoundedRect sets pixels inside a rounded rectangle directly (no blend)
// so the alpha is preserved for later compositing.
func fillRoundedRect(img *image.RGBA, x0, y0, x1, y1, r int, col color.RGBA) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if inRoundedRect(x, y, x0, y0, x1, y1, r) {
				img.SetRGBA(x, y, col)
			}
		}
	}
}

func inRoundedRect(x, y, x0, y0, x1, y1, r int) bool {
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return false
	}
	cx, cy := -1, -1
	if x < x0+r {
		cx = x0 + r
	} else if x > x1-r {
		cx = x1 - r
	}
	if y < y0+r {
		cy = y0 + r
	} else if y > y1-r {
		cy = y1 - r
	}
	if cx == -1 || cy == -1 {
		return true
	}
	dx, dy := float64(x-cx), float64(y-cy)
	return math.Sqrt(dx*dx+dy*dy) <= float64(r)
}
