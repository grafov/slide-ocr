// Package pipeline runs silent dedup, sequential VLM OCR and text merge.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"image"
	"strings"

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
	Snapshot      func() []session.Slide
	Prompt        string
	Illustrations bool
	EmptyAsImages bool
	PDQThreshold  int
	Recognizer    Recognizer
	OnProgress    func(Progress)
	OnSlide       func(markdown.Slide)
	SetStatus     func(id string, status session.Status, detail string)
}

// OneRequest is OCR of a single slide with the current backend settings.
type OneRequest struct {
	Slide         session.Slide
	Index         int
	Prompt        string
	Illustrations bool
	EmptyAsImages bool
	Recognizer    Recognizer
	SetStatus     func(id string, status session.Status, detail string)
}

// Result is OCR output for stitching and saving.
type Result struct {
	Slides []markdown.Slide
}

type runState struct {
	status   map[string]session.Status
	included map[string]bool
	text     map[string]string
}

func newRunState() *runState {
	return &runState{
		status:   map[string]session.Status{},
		included: map[string]bool{},
		text:     map[string]string{},
	}
}

func (st *runState) apply(s session.Slide) session.Slide {
	if v, ok := st.status[s.ID]; ok {
		s.Status = v
	}
	if v, ok := st.included[s.ID]; ok {
		s.Included = v
	}
	if v, ok := st.text[s.ID]; ok {
		s.Text = v
	}
	return s
}

func (st *runState) setStatus(id string, status session.Status, detail string, hook func(string, session.Status, string)) {
	st.status[id] = status
	if hook != nil {
		hook(id, status, detail)
	}
}

// ShouldProcess reports whether the slide still needs a VLM call.
func ShouldProcess(s session.Slide) bool {
	if !s.Included {
		return false
	}
	if s.Status == session.StatusSkipped || s.Status == session.StatusNoText {
		return false
	}
	if s.Status == session.StatusDone && strings.TrimSpace(s.Text) != "" {
		return false
	}
	return true
}

func (req Request) rawSlides() []session.Slide {
	if req.Snapshot != nil {
		return req.Snapshot()
	}
	return req.Slides
}

func (req Request) slides(st *runState) []session.Slide {
	raw := req.rawSlides()
	out := make([]session.Slide, len(raw))
	for i, s := range raw {
		out[i] = st.apply(s)
	}
	return out
}

// Run executes visual dedup, sequential recognition and overlapping-text merge.
func Run(ctx context.Context, req Request) (Result, error) {
	if req.Recognizer == nil {
		return Result{}, fmt.Errorf("recognizer is nil")
	}
	st := newRunState()
	results := map[string]markdown.Slide{}
	imgs := map[string]image.Image{}
	ocrKept := map[string]struct{}{}
	attempted := map[string]struct{}{}
	load := func(s session.Slide) (image.Image, error) {
		if img, ok := imgs[s.ID]; ok {
			return img, nil
		}
		img, err := imageutil.OpenFile(s.Path)
		if err != nil {
			return nil, err
		}
		imgs[s.ID] = img
		return img, nil
	}

	for _, s := range req.slides(st) {
		md := markdown.Slide{ID: s.ID, Name: s.Name, Text: s.Text}
		if !s.Included && s.Status != session.StatusNoText {
			md.Skip = true
			md.Text = ""
		}
		results[s.ID] = md
	}

	for {
		if err := ctx.Err(); err != nil {
			return collectResult(req.slides(st), results), err
		}
		snap := req.slides(st)
		var candidate *session.Slide
		idx := -1
		doneN, todoN := 0, 0
		for i := range snap {
			s := snap[i]
			if !s.Included {
				continue
			}
			if ShouldProcess(s) {
				todoN++
				if candidate == nil {
					if _, ok := attempted[s.ID]; !ok {
						c := s
						candidate = &c
						idx = i
					}
				}
				continue
			}
			doneN++
		}
		total := doneN + todoN
		if candidate == nil {
			return collectResult(snap, results), nil
		}
		s := *candidate
		img, err := load(s)
		if err != nil {
			attempted[s.ID] = struct{}{}
			st.setStatus(s.ID, session.StatusError, err.Error(), req.SetStatus)
			md := markdown.Slide{ID: s.ID, Name: s.Name, Skip: true}
			results[s.ID] = md
			continue
		}
		if skip, dist, ok := visualDup(img, req.PDQThreshold, snap, s.ID, ocrKept, load); ok && skip {
			attempted[s.ID] = struct{}{}
			detail := fmt.Sprintf("дубль кадра (PDQ %d)", dist)
			st.setStatus(s.ID, session.StatusSkipped, detail, req.SetStatus)
			md := markdown.Slide{ID: s.ID, Name: s.Name, Skip: true}
			results[s.ID] = md
			if req.OnSlide != nil {
				req.OnSlide(cloneSlide(md))
			}
			if req.OnProgress != nil {
				req.OnProgress(Progress{Current: doneN, Total: total, Slide: s, Message: detail})
			}
			continue
		}
		st.setStatus(s.ID, session.StatusRunning, "", req.SetStatus)
		if req.OnProgress != nil {
			req.OnProgress(Progress{
				Current: doneN,
				Total:   total,
				Slide:   s,
				Message: "распознавание " + s.Name,
			})
		}
		md, err := recognizeImage(ctx, req.Recognizer, img, req.Prompt, req.Illustrations, req.EmptyAsImages, idx, s)
		attempted[s.ID] = struct{}{}
		if err != nil {
			if canceled(ctx, err) {
				st.setStatus(s.ID, session.StatusPending, "", req.SetStatus)
				return collectResult(req.slides(st), results), err
			}
			st.setStatus(s.ID, session.StatusError, err.Error(), req.SetStatus)
			md = markdown.Slide{ID: s.ID, Name: s.Name, Skip: true}
			results[s.ID] = md
			continue
		}
		if md.NoText {
			st.included[s.ID] = false
			st.text[s.ID] = md.Text
			st.setStatus(s.ID, session.StatusNoText, "Нет текста", req.SetStatus)
			results[s.ID] = md
			if req.OnSlide != nil {
				req.OnSlide(cloneSlide(md))
			}
			continue
		}
		ocrKept[s.ID] = struct{}{}
		st.text[s.ID] = md.Text
		results[s.ID] = md
		skipped := collapseByOrder(results, req.slides(st), func(id string, status session.Status, detail string) {
			st.setStatus(id, status, detail, req.SetStatus)
			if status == session.StatusSkipped {
				delete(st.text, id)
			}
		})
		md = results[s.ID]
		if md.Skip {
			st.setStatus(s.ID, session.StatusSkipped, "повтор текста", req.SetStatus)
		} else {
			st.setStatus(s.ID, session.StatusDone, "", req.SetStatus)
		}
		if req.OnSlide != nil {
			req.OnSlide(cloneSlide(md))
			for _, id := range skipped {
				if id == s.ID {
					continue
				}
				req.OnSlide(cloneSlide(results[id]))
			}
		}
		if req.OnProgress != nil {
			req.OnProgress(Progress{Current: doneN + 1, Total: total, Slide: s, Message: "готово " + s.Name})
		}
	}
}

func collectResult(snap []session.Slide, results map[string]markdown.Slide) Result {
	out := make([]markdown.Slide, 0, len(snap))
	for _, s := range snap {
		if md, ok := results[s.ID]; ok {
			out = append(out, md)
			continue
		}
		md := markdown.Slide{ID: s.ID, Name: s.Name, Text: s.Text}
		if !s.Included && s.Status != session.StatusNoText {
			md.Skip = true
			md.Text = ""
		}
		out = append(out, md)
	}
	return Result{Slides: out}
}

func visualDup(img image.Image, threshold int, snap []session.Slide, id string, ocrKept map[string]struct{}, load func(session.Slide) (image.Image, error)) (skip bool, dist int, ok bool) {
	keep, err := dedup.NewVisualKeep()
	if err != nil {
		return false, 0, false
	}
	for _, other := range snap {
		if other.ID == id {
			continue
		}
		_, ocr := ocrKept[other.ID]
		if !(other.Included && other.Status == session.StatusDone) && !ocr {
			continue
		}
		oimg, oerr := load(other)
		if oerr != nil {
			continue
		}
		keep.Remember(oimg)
	}
	skip, dist = keep.Duplicate(img, threshold)
	return skip, dist, true
}

func collapseByOrder(results map[string]markdown.Slide, snap []session.Slide, setStatus func(id string, status session.Status, detail string)) []string {
	type item struct {
		id string
		md markdown.Slide
	}
	var chain []item
	for _, s := range snap {
		md, ok := results[s.ID]
		if !ok || md.Skip || md.NoText || md.Text == "" {
			continue
		}
		chain = append(chain, item{id: s.ID, md: md})
	}
	var newly []string
	last := -1
	for i := range chain {
		if last >= 0 && dedup.Overlaps(chain[last].md.Text, chain[i].md.Text) {
			if len([]rune(chain[i].md.Text)) >= len([]rune(chain[last].md.Text)) {
				old := chain[last]
				old.md.Skip = true
				old.md.Text = ""
				results[old.id] = old.md
				if setStatus != nil {
					setStatus(old.id, session.StatusSkipped, "повтор текста")
				}
				newly = append(newly, old.id)
				last = i
			} else {
				chain[i].md.Skip = true
				chain[i].md.Text = ""
				results[chain[i].id] = chain[i].md
				if setStatus != nil {
					setStatus(chain[i].id, session.StatusSkipped, "повтор текста")
				}
				newly = append(newly, chain[i].id)
			}
			continue
		}
		last = i
	}
	return newly
}

// RecognizeOne runs OCR for a single slide, replacing illustration crops.
func RecognizeOne(ctx context.Context, req OneRequest) (markdown.Slide, error) {
	if req.Recognizer == nil {
		return markdown.Slide{}, fmt.Errorf("recognizer is nil")
	}
	s := req.Slide
	if req.SetStatus != nil {
		req.SetStatus(s.ID, session.StatusRunning, "")
	}
	img, err := imageutil.OpenFile(s.Path)
	if err != nil {
		if req.SetStatus != nil {
			req.SetStatus(s.ID, session.StatusError, err.Error())
		}
		return markdown.Slide{ID: s.ID, Name: s.Name, Skip: true}, err
	}
	md, err := recognizeImage(ctx, req.Recognizer, img, req.Prompt, req.Illustrations, req.EmptyAsImages, req.Index, s)
	if err != nil {
		if canceled(ctx, err) {
			if req.SetStatus != nil {
				req.SetStatus(s.ID, session.StatusPending, "")
			}
			return markdown.Slide{ID: s.ID, Name: s.Name}, err
		}
		if req.SetStatus != nil {
			req.SetStatus(s.ID, session.StatusError, err.Error())
		}
		return markdown.Slide{ID: s.ID, Name: s.Name, Skip: true}, err
	}
	if md.NoText {
		if req.SetStatus != nil {
			req.SetStatus(s.ID, session.StatusNoText, "Нет текста")
		}
		return md, nil
	}
	if req.SetStatus != nil {
		req.SetStatus(s.ID, session.StatusDone, "")
	}
	return md, nil
}

func recognizeImage(ctx context.Context, rec Recognizer, img image.Image, prompt string, illustrations, emptyAsImages bool, index int, s session.Slide) (markdown.Slide, error) {
	if err := ctx.Err(); err != nil {
		return markdown.Slide{}, err
	}
	store := &imageutil.AssetStore{
		Prefix: fmt.Sprintf("slide-%03d", index+1),
		Source: img,
	}
	text, err := rec.Recognize(ctx, img, prompt, illustrations, store)
	if err != nil {
		return markdown.Slide{ID: s.ID, Name: s.Name, Skip: true}, err
	}
	md := markdown.Slide{
		ID:     s.ID,
		Name:   s.Name,
		Text:   text,
		Assets: store.Items,
	}
	if markdown.HasSlideText(text) {
		return md, nil
	}
	md.NoText = true
	if emptyAsImages {
		rel, err := store.SaveFull("Нет текста")
		if err == nil {
			md.Text = fmt.Sprintf("![Нет текста](%s)", rel)
			md.Assets = store.Items
		} else {
			md.Text = ""
			md.Assets = nil
		}
	} else {
		md.Text = ""
		md.Assets = nil
	}
	return md, nil
}

func canceled(ctx context.Context, err error) bool {
	return errors.Is(err, context.Canceled) || ctx.Err() != nil
}

func cloneSlide(s markdown.Slide) markdown.Slide {
	if len(s.Assets) > 0 {
		s.Assets = append([]imageutil.Asset(nil), s.Assets...)
	}
	return s
}
