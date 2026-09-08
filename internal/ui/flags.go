package ui

import (
	"strings"

	"github.com/grafov/slide-ocr/internal/markdown"
)

func shouldOpenDialog(startID, lastID int) bool {
	return startID >= 0 && startID == lastID
}

func hasUnsaved(slides []markdown.Slide, saved map[string]bool) bool {
	for _, s := range slides {
		if s.Skip || strings.TrimSpace(s.Text) == "" {
			continue
		}
		if !saved[s.ID] {
			return true
		}
	}
	return false
}
