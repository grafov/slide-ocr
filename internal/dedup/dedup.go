// Package dedup clusters near-duplicate slides and merges overlapping OCR text.
package dedup

import (
	"image"
	"strings"
	"unicode"

	"github.com/ajdnik/imghash/v2"
)

// DefaultPDQThreshold is a strict Hamming distance for PDQ (256-bit).
// Meta uses 31 for near-duplicates; slide templates share chrome, so we stay lower.
const DefaultPDQThreshold = 12

// Decision says whether a slide should be OCR'd or skipped as a visual duplicate.
type Decision struct {
	Index       int
	Skip        bool
	DuplicateOf int
	Distance    int
	HashError   error
}

// PlanVisual marks later near-duplicates of earlier slides in the given order.
func PlanVisual(imgs []image.Image, threshold int) []Decision {
	out := make([]Decision, len(imgs))
	if threshold <= 0 {
		threshold = DefaultPDQThreshold
	}
	hasher, err := imghash.NewPDQ()
	if err != nil {
		for i := range imgs {
			out[i] = Decision{Index: i, HashError: err}
		}
		return out
	}
	type hashed struct {
		idx  int
		hash imghash.Hash
		ok   bool
	}
	kept := make([]hashed, 0, len(imgs))
	for i, img := range imgs {
		dec := Decision{Index: i, DuplicateOf: -1}
		if img == nil {
			out[i] = dec
			continue
		}
		h, herr := hasher.Calculate(img)
		if herr != nil {
			dec.HashError = herr
			out[i] = dec
			continue
		}
		skip := false
		for _, prev := range kept {
			dist, cerr := hasher.Compare(h, prev.hash)
			if cerr != nil {
				continue
			}
			d := int(dist + 0.5)
			if d <= threshold {
				dec.Skip = true
				dec.DuplicateOf = prev.idx
				dec.Distance = d
				skip = true
				break
			}
		}
		if !skip {
			kept = append(kept, hashed{idx: i, hash: h, ok: true})
		}
		out[i] = dec
	}
	return out
}

// VisualKeep is an incremental PDQ index of frames already kept.
type VisualKeep struct {
	pdq  imghash.PDQ
	kept []imghash.Hash
}

// NewVisualKeep constructs a PDQ hasher for live dedup.
func NewVisualKeep() (*VisualKeep, error) {
	p, err := imghash.NewPDQ()
	if err != nil {
		return nil, err
	}
	return &VisualKeep{pdq: p}, nil
}

// Remember adds img to the set of kept frames.
func (v *VisualKeep) Remember(img image.Image) {
	if v == nil || img == nil {
		return
	}
	h, err := v.pdq.Calculate(img)
	if err != nil {
		return
	}
	v.kept = append(v.kept, h)
}

// Duplicate reports whether img is a near-duplicate of a kept frame.
func (v *VisualKeep) Duplicate(img image.Image, threshold int) (bool, int) {
	if v == nil || img == nil {
		return false, 0
	}
	if threshold <= 0 {
		threshold = DefaultPDQThreshold
	}
	h, err := v.pdq.Calculate(img)
	if err != nil {
		return false, 0
	}
	for _, prev := range v.kept {
		dist, cerr := v.pdq.Compare(h, prev)
		if cerr != nil {
			continue
		}
		d := int(dist + 0.5)
		if d <= threshold {
			return true, d
		}
	}
	return false, 0
}

// MergeOverlapping keeps the fuller of two consecutive near-duplicate OCR texts.
func MergeOverlapping(texts []string) []string {
	if len(texts) == 0 {
		return texts
	}
	out := []string{texts[0]}
	for i := 1; i < len(texts); i++ {
		cur := texts[i]
		prev := out[len(out)-1]
		if isOverlap(prev, cur) {
			if len([]rune(strings.TrimSpace(cur))) >= len([]rune(strings.TrimSpace(prev))) {
				out[len(out)-1] = cur
			}
			continue
		}
		out = append(out, cur)
	}
	return out
}

// Overlaps reports whether two OCR texts are the same slide shown twice or incrementally.
func Overlaps(a, b string) bool {
	return isOverlap(a, b)
}

func isOverlap(a, b string) bool {
	na, nb := normalize(a), normalize(b)
	if na == "" || nb == "" {
		return false
	}
	if na == nb {
		return true
	}
	if strings.Contains(na, nb) || strings.Contains(nb, na) {
		shorter := na
		if len(nb) < len(na) {
			shorter = nb
		}
		return len(shorter) >= 40 || float64(len(shorter))/float64(max(len(na), len(nb))) > 0.65
	}
	wa, wb := words(na), words(nb)
	if len(wa) < 4 || len(wb) < 4 {
		return false
	}
	j := jaccard(wa, wb)
	if j >= 0.88 {
		return true
	}
	return prefixOverlap(wa, wb) >= 0.8
}

func normalize(s string) string {
	var b strings.Builder
	prevSpace := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}

func words(s string) []string {
	return strings.Fields(s)
}

func jaccard(a, b []string) float64 {
	set := make(map[string]struct{}, len(a))
	for _, w := range a {
		set[w] = struct{}{}
	}
	inter := 0
	for _, w := range b {
		if _, ok := set[w]; ok {
			inter++
		}
	}
	union := len(set)
	for _, w := range b {
		if _, ok := set[w]; !ok {
			union++
			set[w] = struct{}{}
		}
	}
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func prefixOverlap(a, b []string) float64 {
	n := min(len(a), len(b))
	same := 0
	for i := 0; i < n; i++ {
		if a[i] == b[i] {
			same++
			continue
		}
		break
	}
	if n == 0 {
		return 0
	}
	return float64(same) / float64(n)
}
