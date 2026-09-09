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
		if !slideInOutput(s) {
			continue
		}
		if !saved[s.ID] {
			return true
		}
	}
	return false
}

func slideInOutput(s markdown.Slide) bool {
	if s.Skip {
		return false
	}
	if s.NoText {
		return true
	}
	return strings.TrimSpace(s.Text) != ""
}
