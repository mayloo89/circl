package albums

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/webp"
)

var wmFace font.Face

func init() {
	tt, err := opentype.Parse(gomono.TTF)
	if err != nil {
		panic("albums: parse gomono: " + err.Error())
	}
	wmFace, err = opentype.NewFace(tt, &opentype.FaceOptions{Size: 11, DPI: 96})
	if err != nil {
		panic("albums: new watermark face: " + err.Error())
	}
}

// compositeWatermark overlays a semi-transparent pill label
// ("<uid[:8]> · MM-DD HH:MM") at the bottom-right of the image
// so unauthorized redistribution can be traced back to a specific viewer.
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
	draw.Draw(out, bounds, img, bounds.Min, draw.Src)

	shortUID := uid
	if len(shortUID) > 8 {
		shortUID = shortUID[:8]
	}
	label := shortUID + " · " + t.UTC().Format("01-02 15:04")
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
	wmPadH   = 8  // horizontal padding inside pill
	wmPadV   = 4  // vertical padding inside pill
	wmRadius = 4  // corner radius
	wmMargin = 10 // distance from image edge
)

func drawWMPill(img *image.RGBA, label string, w, h int) {
	metrics := wmFace.Metrics()
	d := &font.Drawer{Face: wmFace}
	textW := int(d.MeasureString(label) >> 6)
	textH := int((metrics.Ascent+metrics.Descent)>>6) + 1

	pillW := textW + 2*wmPadH
	pillH := textH + 2*wmPadV
	x1 := w - wmMargin
	y1 := h - wmMargin
	x0 := x1 - pillW
	y0 := y1 - pillH

	fillRoundedRect(img, x0, y0, x1, y1, wmRadius, color.RGBA{0, 0, 0, 140})

	d = &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{255, 255, 255, 230}),
		Face: wmFace,
		Dot: fixed.Point26_6{
			X: fixed.I(x0 + wmPadH),
			Y: fixed.I(y0+wmPadV) + metrics.Ascent,
		},
	}
	d.DrawString(label)
}

func fillRoundedRect(img *image.RGBA, x0, y0, x1, y1, r int, col color.RGBA) {
	a := float64(col.A) / 255.0
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !inRoundedRect(x, y, x0, y0, x1, y1, r) {
				continue
			}
			dst := img.RGBAAt(x, y)
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(float64(col.R)*a + float64(dst.R)*(1-a)),
				G: uint8(float64(col.G)*a + float64(dst.G)*(1-a)),
				B: uint8(float64(col.B)*a + float64(dst.B)*(1-a)),
				A: 255,
			})
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
