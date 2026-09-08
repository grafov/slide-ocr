package imageutil

import (
	"fmt"
	"image"
	"strings"
)

const assetsDir = "assets"

// Asset is a cropped or saved illustration to embed in Markdown.
type Asset struct {
	FileName string
	RelPath  string
	Caption  string
	Data     []byte
}

// AssetStore collects illustrations for one slide.
type AssetStore struct {
	Prefix string
	Source image.Image
	Items  []Asset
}

// CropRegion saves a fractional crop and returns a markdown-relative path.
func (s *AssetStore) CropRegion(x, y, w, h float64, caption string) (string, error) {
	if s.Source == nil {
		return "", fmt.Errorf("no source image")
	}
	cropped, err := CropFraction(s.Source, x, y, w, h)
	if err != nil {
		return "", err
	}
	return s.saveImage(cropped, caption)
}

// SaveFull stores the whole slide as an illustration.
func (s *AssetStore) SaveFull(caption string) (string, error) {
	if s.Source == nil {
		return "", fmt.Errorf("no source image")
	}
	return s.saveImage(s.Source, caption)
}

func (s *AssetStore) saveImage(img image.Image, caption string) (string, error) {
	data, err := EncodePNG(img)
	if err != nil {
		return "", err
	}
	n := len(s.Items) + 1
	prefix := s.Prefix
	if prefix == "" {
		prefix = "slide"
	}
	name := fmt.Sprintf("%s-fig-%d.png", prefix, n)
	rel := assetsDir + "/" + name
	s.Items = append(s.Items, Asset{
		FileName: name,
		RelPath:  rel,
		Caption:  strings.TrimSpace(caption),
		Data:     data,
	})
	return rel, nil
}
