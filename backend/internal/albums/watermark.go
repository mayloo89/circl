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
	wmFace, err = opentype.NewFace(tt, &opentype.FaceOptions{Size: 13, DPI: 96})
	if err != nil {
		panic("albums: new watermark face: " + err.Error())
	}
}

// compositeWatermark overlays a semi-transparent deterrence label
// ("<displayName> · YYYY-MM-DD HH:MM UTC") at the bottom-right of the image
// so unauthorized redistribution can be traced back to a specific viewer.
// WebP input is decoded and re-encoded as JPEG (WebP encoding requires cgo).
// Unrecognised content types are returned as-is without modification.
func compositeWatermark(src io.Reader, contentType, displayName string, t time.Time) ([]byte, string, error) {
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

	label := fmt.Sprintf("%s · %s", displayName, t.UTC().Format("2006-01-02 15:04 UTC"))
	drawWMText(out, label, bounds.Dx(), bounds.Dy(), color.RGBA{0, 0, 0, 160}, 1)
	drawWMText(out, label, bounds.Dx(), bounds.Dy(), color.RGBA{255, 255, 255, 200}, 0)

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

// drawWMText renders label in the bottom-right corner of img.
// offset shifts the text by (offset, offset) pixels for the shadow pass.
func drawWMText(img *image.RGBA, label string, w, h int, col color.Color, offset int) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: wmFace,
	}
	advance := d.MeasureString(label)
	x := fixed.I(w) - advance - fixed.I(10+offset)
	y := fixed.I(h-10) - fixed.I(offset)
	if x < fixed.I(4) {
		x = fixed.I(4)
	}
	d.Dot = fixed.Point26_6{X: x, Y: y}
	d.DrawString(label)
}
