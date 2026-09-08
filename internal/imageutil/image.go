// Package imageutil decodes, thumbnails and crops slide images.
package imageutil

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/kovidgoyal/imaging"
)

const (
	// MaxLLMSide is the longest edge sent to a vision model.
	MaxLLMSide = 2048
	// MaxPreviewSide is the longest edge of the right-click preview dialog.
	MaxPreviewSide = 1600
	jpegQual       = 85
)

var imageExt = map[string]struct{}{
	".jpg": {}, ".jpeg": {}, ".png": {}, ".webp": {},
	".tif": {}, ".tiff": {}, ".gif": {}, ".bmp": {},
}

// IsImagePath reports whether path has a supported image extension.
func IsImagePath(path string) bool {
	_, ok := imageExt[strings.ToLower(filepath.Ext(path))]
	return ok
}

// OpenFile decodes an image using the pure-Go backend and EXIF orientation.
func OpenFile(path string) (image.Image, error) {
	return imaging.Open(path, imaging.Backends(imaging.GO_IMAGE))
}

// ThumbnailFile builds a square preview of the given size in pixels.
func ThumbnailFile(path string, size int) (image.Image, error) {
	img, err := OpenFile(path)
	if err != nil {
		return nil, err
	}
	return imaging.Thumbnail(img, size, size, imaging.Linear), nil
}

// FitForPreview downscales a large image for the on-screen preview dialog.
func FitForPreview(img image.Image) image.Image {
	b := img.Bounds()
	if b.Dx() <= MaxPreviewSide && b.Dy() <= MaxPreviewSide {
		return img
	}
	return imaging.Fit(img, MaxPreviewSide, MaxPreviewSide, imaging.Lanczos)
}

// PrepareForLLM resizes large slides and encodes JPEG bytes for a data URI.
func PrepareForLLM(img image.Image) (mime string, data []byte, err error) {
	fitted := img
	b := img.Bounds()
	if b.Dx() > MaxLLMSide || b.Dy() > MaxLLMSide {
		fitted = imaging.Fit(img, MaxLLMSide, MaxLLMSide, imaging.Lanczos)
	}
	var buf bytes.Buffer
	if err = imaging.Encode(&buf, fitted, imaging.JPEG, imaging.JPEGQuality(jpegQual)); err != nil {
		buf.Reset()
		if err = imaging.Encode(&buf, fitted, imaging.PNG); err != nil {
			return "", nil, err
		}
		return "image/png", buf.Bytes(), nil
	}
	return "image/jpeg", buf.Bytes(), nil
}

// CropFraction cuts a region given as fractions of width/height in 0..1.
func CropFraction(img image.Image, x, y, w, h float64) (image.Image, error) {
	x, y, w, h = clamp01(x), clamp01(y), clamp01(w), clamp01(h)
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("empty crop region")
	}
	b := img.Bounds()
	px := b.Min.X + int(float64(b.Dx())*x)
	py := b.Min.Y + int(float64(b.Dy())*y)
	pw := int(float64(b.Dx()) * w)
	ph := int(float64(b.Dy()) * h)
	if pw < 1 {
		pw = 1
	}
	if ph < 1 {
		ph = 1
	}
	rect := image.Rect(px, py, px+pw, py+ph)
	cropped := imaging.Crop(img, rect)
	if cropped.Bounds().Empty() {
		return nil, fmt.Errorf("crop produced an empty image")
	}
	return cropped, nil
}

// EncodePNG writes img as PNG.
func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, img, imaging.PNG); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteFile writes data to path, creating parent directories.
func WriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
