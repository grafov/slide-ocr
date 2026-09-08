package markdown

import (
	"regexp"
	"unicode"
)

var imageLink = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)

// HasSlideText reports whether OCR output contains letters or digits besides image links.
func HasSlideText(s string) bool {
	s = imageLink.ReplaceAllString(s, " ")
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// OrderByIDs returns slides in the given id order, omitting unknown ids.
func OrderByIDs(ids []string, slides []Slide) []Slide {
	byID := make(map[string]Slide, len(slides))
	for _, s := range slides {
		byID[s.ID] = s
	}
	out := make([]Slide, 0, len(ids))
	for _, id := range ids {
		if s, ok := byID[id]; ok {
			out = append(out, s)
		}
	}
	return out
}
