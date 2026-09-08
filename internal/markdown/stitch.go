// Package markdown stitches slide OCR results and writes files plus sidecar assets.
package markdown

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grafov/slide-ocr/internal/imageutil"
)

// Mode selects how slides are joined on save.
type Mode int

const (
	// ModeContinuous joins slides with blank lines only.
	ModeContinuous Mode = iota
	// ModeSeparator inserts ----- between slides.
	ModeSeparator
	// ModeSeparateFiles writes one markdown file per slide.
	ModeSeparateFiles
)

const (
	separator    = "-----"
	assetsMarker = "assets/"
	// CombinedFileName is the Markdown file written for continuous and separator modes.
	CombinedFileName = "recognized-text.md"
)

// Slide is one recognized slide ready for export.
type Slide struct {
	ID     string
	Name   string
	Text   string
	Assets []imageutil.Asset
	Skip   bool
}

// Stitch builds a single markdown document according to mode.
func Stitch(slides []Slide, mode Mode) string {
	var parts []string
	for _, s := range slides {
		if s.Skip || strings.TrimSpace(s.Text) == "" {
			continue
		}
		parts = append(parts, strings.TrimSpace(s.Text))
	}
	if mode == ModeSeparator {
		return strings.Join(parts, "\n\n"+separator+"\n\n") + trailing(parts)
	}
	return strings.Join(parts, "\n\n") + trailing(parts)
}

func trailing(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return "\n"
}

// Save writes markdown and illustration files.
// For ModeContinuous and ModeSeparator dest is a .md file path.
// For ModeSeparateFiles dest is a directory.
func Save(slides []Slide, mode Mode, dest string) error {
	switch mode {
	case ModeSeparateFiles:
		return saveSeparate(slides, dest)
	default:
		body := Stitch(slides, mode)
		stem := strings.TrimSuffix(filepath.Base(dest), filepath.Ext(dest))
		assetDirName := stem + ".assets"
		body = rewriteAssets(body, assetDirName)
		if err := imageutil.WriteFile(dest, []byte(body)); err != nil {
			return err
		}
		dir := filepath.Join(filepath.Dir(dest), assetDirName)
		return writeAssets(slides, dir)
	}
}

// SaveToDir writes the current slides into dir, overwriting previous files of the same names.
func SaveToDir(slides []Slide, mode Mode, dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("empty output directory")
	}
	if mode == ModeSeparateFiles {
		return saveSeparate(slides, dir)
	}
	return Save(slides, mode, filepath.Join(dir, CombinedFileName))
}

func saveSeparate(slides []Slide, dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	assetDir := filepath.Join(dir, "assets")
	used := make(map[string]int)
	for _, s := range slides {
		if s.Skip || strings.TrimSpace(s.Text) == "" {
			continue
		}
		name := uniqueMarkdownName(used, s.Name)
		path := filepath.Join(dir, name)
		text := strings.TrimSpace(s.Text) + "\n"
		if err := imageutil.WriteFile(path, []byte(text)); err != nil {
			return err
		}
	}
	return writeAssets(slides, assetDir)
}

func uniqueMarkdownName(used map[string]int, name string) string {
	stem := sanitize(name)
	used[stem]++
	n := used[stem]
	if n == 1 {
		return stem + ".md"
	}
	return fmt.Sprintf("%s-%d.md", stem, n)
}

func writeAssets(slides []Slide, dir string) error {
	for _, s := range slides {
		for _, a := range s.Assets {
			if len(a.Data) == 0 {
				continue
			}
			if err := imageutil.WriteFile(filepath.Join(dir, a.FileName), a.Data); err != nil {
				return err
			}
		}
	}
	return nil
}

func rewriteAssets(body, assetDirName string) string {
	return strings.ReplaceAll(body, "]("+assetsMarker, "]("+assetDirName+"/")
}

func sanitize(name string) string {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	var b strings.Builder
	for _, r := range base {
		if r == ' ' {
			b.WriteByte('-')
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "slide"
	}
	return out
}
