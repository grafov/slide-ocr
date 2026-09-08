// Package pipeline runs silent dedup, sequential VLM OCR and text merge.
package pipeline

import (
	"context"
	"fmt"
	"image"

	"github.com/grafov/slide-ocr/internal/dedup"
	"github.com/grafov/slide-ocr/internal/imageutil"
	"github.com/grafov/slide-ocr/internal/markdown"
	"github.com/grafov/slide-ocr/internal/session"
)

// Recognizer turns a single slide image into Markdown.
type Recognizer interface {
	Recognize(ctx context.Context, img image.Image, prompt string, illustrations bool, store *imageutil.AssetStore) (string, error)
}

// Progress reports run status for the UI.
type Progress struct {
	Current int
	Total   int
	Slide   session.Slide
	Message string
}

// Request is a full recognition run.
type Request struct {
	Slides        []session.Slide
	Prompt        string
	Illustrations bool
	PDQThreshold  int
	Recognizer    Recognizer
	OnProgress    func(Progress)
	SetStatus     func(id string, status session.Status, detail string)
}

// Result is OCR output for stitching and saving.
type Result struct {
	Slides []markdown.Slide
}

// Run executes visual dedup, sequential recognition and overlapping-text merge.
func Run(ctx context.Context, req Request) (Result, error) {
	if req.Recognizer == nil {
		return Result{}, fmt.Errorf("recognizer is nil")
	}
	n := len(req.Slides)
	imgs := make([]image.Image, n)
	for i, s := range req.Slides {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		img, err := imageutil.OpenFile(s.Path)
		if err != nil {
			imgs[i] = nil
			if req.SetStatus != nil {
				req.SetStatus(s.ID, session.StatusError, err.Error())
			}
			continue
		}
		imgs[i] = img
	}

	plan := dedup.PlanVisual(imgs, req.PDQThreshold)
	out := make([]markdown.Slide, n)

	todo := 0
	for _, d := range plan {
		if !d.Skip && imgs[d.Index] != nil {
			todo++
		}
	}
	done := 0

	for i, s := range req.Slides {
		out[i] = markdown.Slide{Name: s.Name}
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		if imgs[i] == nil {
			out[i].Skip = true
			continue
		}
		if plan[i].Skip {
			detail := fmt.Sprintf("дубль кадра (PDQ %d)", plan[i].Distance)
			out[i].Skip = true
			if req.SetStatus != nil {
				req.SetStatus(s.ID, session.StatusSkipped, detail)
			}
			if req.OnProgress != nil {
				req.OnProgress(Progress{Current: done, Total: todo, Slide: s, Message: detail})
			}
			continue
		}
		if req.SetStatus != nil {
			req.SetStatus(s.ID, session.StatusRunning, "")
		}
		if req.OnProgress != nil {
			req.OnProgress(Progress{
				Current: done,
				Total:   todo,
				Slide:   s,
				Message: "распознавание " + s.Name,
			})
		}
		store := &imageutil.AssetStore{
			Prefix: fmt.Sprintf("slide-%03d", i+1),
			Source: imgs[i],
		}
		text, err := req.Recognizer.Recognize(ctx, imgs[i], req.Prompt, req.Illustrations, store)
		if err != nil {
			if req.SetStatus != nil {
				req.SetStatus(s.ID, session.StatusError, err.Error())
			}
			out[i].Skip = true
			done++
			continue
		}
		out[i].Text = text
		out[i].Assets = store.Items
		if req.SetStatus != nil {
			req.SetStatus(s.ID, session.StatusDone, "")
		}
		done++
		if req.OnProgress != nil {
			req.OnProgress(Progress{Current: done, Total: todo, Slide: s, Message: "готово " + s.Name})
		}
	}

	collapseTextDuplicates(out, req)

	return Result{Slides: out}, nil
}

func collapseTextDuplicates(out []markdown.Slide, req Request) {
	last := -1
	for i := range out {
		if out[i].Skip || out[i].Text == "" {
			continue
		}
		if last >= 0 && dedup.Overlaps(out[last].Text, out[i].Text) {
			if len([]rune(out[i].Text)) >= len([]rune(out[last].Text)) {
				out[last].Skip = true
				out[last].Text = ""
				if req.SetStatus != nil && last < len(req.Slides) {
					req.SetStatus(req.Slides[last].ID, session.StatusSkipped, "повтор текста")
				}
				last = i
			} else {
				out[i].Skip = true
				out[i].Text = ""
				if req.SetStatus != nil && i < len(req.Slides) {
					req.SetStatus(req.Slides[i].ID, session.StatusSkipped, "повтор текста")
				}
			}
			continue
		}
		last = i
	}
}
