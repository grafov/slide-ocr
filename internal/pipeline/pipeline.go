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
	OnSlide       func(markdown.Slide)
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
	out := make([]markdown.Slide, n)
	imgs := make([]image.Image, n)
	for i, s := range req.Slides {
		if err := ctx.Err(); err != nil {
			return Result{Slides: out}, err
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

	todo := 0
	for _, d := range plan {
		if !d.Skip && imgs[d.Index] != nil {
			todo++
		}
	}
	done := 0

	for i, s := range req.Slides {
		out[i] = markdown.Slide{ID: s.ID, Name: s.Name}
		if err := ctx.Err(); err != nil {
			return Result{Slides: out}, err
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
		wasSkip := make([]bool, n)
		for j := range out {
			wasSkip[j] = out[j].Skip
		}
		out[i].Text = text
		out[i].Assets = store.Items
		collapseTextDuplicates(out, req)
		if out[i].Skip {
			if req.SetStatus != nil {
				req.SetStatus(s.ID, session.StatusSkipped, "повтор текста")
			}
		} else if req.SetStatus != nil {
			req.SetStatus(s.ID, session.StatusDone, "")
		}
		emitChanged(req, out, i, wasSkip)
		done++
		if req.OnProgress != nil {
			req.OnProgress(Progress{Current: done, Total: todo, Slide: s, Message: "готово " + s.Name})
		}
	}

	return Result{Slides: out}, nil
}

func emitChanged(req Request, out []markdown.Slide, current int, wasSkip []bool) {
	if req.OnSlide == nil {
		return
	}
	req.OnSlide(cloneSlide(out[current]))
	for j := range out {
		if j == current {
			continue
		}
		if out[j].Skip && (j >= len(wasSkip) || !wasSkip[j]) {
			req.OnSlide(cloneSlide(out[j]))
		}
	}
}

func cloneSlide(s markdown.Slide) markdown.Slide {
	if len(s.Assets) > 0 {
		s.Assets = append([]imageutil.Asset(nil), s.Assets...)
	}
	return s
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
