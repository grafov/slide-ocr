// Package session keeps the ordered list of slide images for the desktop UI.
package session

import (
	"context"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"image"

	"github.com/grafov/slide-ocr/internal/imageutil"
)

// Status is the recognition state of a single slide.
type Status string

const (
	// StatusPending means the slide is waiting for recognition.
	StatusPending Status = "pending"
	// StatusSkipped means the slide was treated as a visual duplicate.
	StatusSkipped Status = "skipped"
	// StatusRunning means the slide is being sent to the VLM.
	StatusRunning Status = "running"
	// StatusDone means recognition finished successfully.
	StatusDone Status = "done"
	// StatusError means recognition failed for this file.
	StatusError Status = "error"
)

// Slide is one image in the current batch.
type Slide struct {
	ID      string
	Path    string
	Name    string
	ModTime time.Time
	Thumb   image.Image
	Status  Status
	Detail  string
	Text    string
}

// Document is the shared, mutex-protected batch used by all tabs.
type Document struct {
	mu     sync.Mutex
	slides []Slide
	nextID atomic.Uint64
}

// Snapshot returns a copy of the current slide list.
func (d *Document) Snapshot() []Slide {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]Slide, len(d.slides))
	copy(out, d.slides)
	return out
}

// Len returns the number of slides.
func (d *Document) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.slides)
}

// At returns the slide at index, or false if out of range.
func (d *Document) At(index int) (Slide, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if index < 0 || index >= len(d.slides) {
		return Slide{}, false
	}
	return d.slides[index], true
}

// AddPaths appends image files that are not already in the list.
func (d *Document) AddPaths(paths []string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	known := make(map[string]struct{}, len(d.slides))
	for _, s := range d.slides {
		known[s.Path] = struct{}{}
	}
	added := 0
	for _, p := range paths {
		p = filepath.Clean(p)
		if !imageutil.IsImagePath(p) {
			continue
		}
		if _, ok := known[p]; ok {
			continue
		}
		infoMod := time.Time{}
		if t, err := imageutil.ModTime(p); err == nil {
			infoMod = t
		}
		d.nextID.Add(1)
		id := d.nextID.Load()
		d.slides = append(d.slides, Slide{
			ID:      strconv.FormatUint(id, 10),
			Path:    p,
			Name:    filepath.Base(p),
			ModTime: infoMod,
			Status:  StatusPending,
		})
		known[p] = struct{}{}
		added++
	}
	return added
}

// Reverse reverses the current order.
func (d *Document) Reverse() {
	d.mu.Lock()
	defer d.mu.Unlock()
	slices.Reverse(d.slides)
}

// SortByName sorts by file name.
func (d *Document) SortByName() {
	d.mu.Lock()
	defer d.mu.Unlock()
	sort.SliceStable(d.slides, func(i, j int) bool {
		return strings.ToLower(d.slides[i].Name) < strings.ToLower(d.slides[j].Name)
	})
}

// SortByModTime sorts by file modification time.
func (d *Document) SortByModTime() {
	d.mu.Lock()
	defer d.mu.Unlock()
	sort.SliceStable(d.slides, func(i, j int) bool {
		return d.slides[i].ModTime.Before(d.slides[j].ModTime)
	})
}

// Move relocates the slide at from to index to.
func (d *Document) Move(from, to int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.slides)
	if from < 0 || from >= n || to < 0 || to >= n || from == to {
		return
	}
	item := d.slides[from]
	d.slides = append(d.slides[:from], d.slides[from+1:]...)
	if to > from {
		to--
	}
	d.slides = slices.Insert(d.slides, to, item)
}

// SwapAdjacent moves the slide at index one step up or down.
func (d *Document) SwapAdjacent(index int, down bool) {
	if down {
		d.Move(index, index+1)
		return
	}
	d.Move(index, index-1)
}

// SetThumb stores a thumbnail for the slide with the given id.
func (d *Document) SetThumb(id string, thumb image.Image) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.slides {
		if d.slides[i].ID == id {
			d.slides[i].Thumb = thumb
			return
		}
	}
}

// SetStatus updates recognition status for a slide id.
func (d *Document) SetStatus(id string, status Status, detail string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.slides {
		if d.slides[i].ID == id {
			d.slides[i].Status = status
			d.slides[i].Detail = detail
			return
		}
	}
}

// SetText stores OCR text for a slide id.
func (d *Document) SetText(id, text string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.slides {
		if d.slides[i].ID == id {
			d.slides[i].Text = text
			d.slides[i].Status = StatusDone
			d.slides[i].Detail = ""
			return
		}
	}
}

// ResetRecognition clears per-slide OCR state before a new run.
func (d *Document) ResetRecognition() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.slides {
		d.slides[i].Status = StatusPending
		d.slides[i].Detail = ""
		d.slides[i].Text = ""
	}
}

// LoadThumbs fills missing thumbnails. each is called after every successful decode.
func (d *Document) LoadThumbs(ctx context.Context, each func()) {
	slides := d.Snapshot()
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, s := range slides {
		if s.Thumb != nil {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		wg.Add(1)
		go func(id, path string) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()
			thumb, err := imageutil.ThumbnailFile(path, thumbSize)
			if err != nil {
				return
			}
			d.SetThumb(id, thumb)
			if each != nil {
				each()
			}
		}(s.ID, s.Path)
	}
	wg.Wait()
}

const thumbSize = 72
