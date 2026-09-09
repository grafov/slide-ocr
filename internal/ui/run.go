package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"github.com/grafov/slide-ocr/internal/dedup"
	"github.com/grafov/slide-ocr/internal/llm"
	"github.com/grafov/slide-ocr/internal/markdown"
	"github.com/grafov/slide-ocr/internal/pipeline"
	"github.com/grafov/slide-ocr/internal/session"
)

func (u *App) start() {
	if u.running {
		return
	}
	if u.doc.Len() == 0 {
		dialog.ShowError(errors.New("нет файлов"), u.win)
		return
	}
	u.savePrefs()
	cfg := u.llmConfig()
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultURL
		u.urlEntry.SetText(cfg.BaseURL)
	}
	if strings.TrimSpace(cfg.Model) == "" {
		dialog.ShowError(errors.New("укажите модель"), u.win)
		return
	}
	promptText := u.promptBox.Text
	if strings.TrimSpace(promptText) == "" {
		u.rebuildPrompt()
		promptText = u.promptBox.Text
	}
	slides := u.doc.Snapshot()
	var work int
	for _, s := range slides {
		if pipeline.ShouldProcess(s) {
			work++
		}
	}
	if work == 0 {
		dialog.ShowError(errors.New("нет слайдов для распознавания"), u.win)
		return
	}
	u.ensureResultSlots(slides)
	u.setFragmentsView(true)
	u.running = true
	u.batchRun = true
	u.runStarted = time.Now()
	u.runProg = pipeline.Progress{}
	u.startBtn.Disable()
	u.pauseBtn.Enable()
	u.refreshList()
	u.startRunTicker()
	u.updateRunChrome()
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	client := llm.New(cfg)
	illustrations := u.optIllust.Checked
	emptyAs := u.optEmptyIllust.Checked
	go func() {
		res, err := pipeline.Run(ctx, pipeline.Request{
			Snapshot:      u.doc.Snapshot,
			Prompt:        promptText,
			Illustrations: illustrations,
			EmptyAsImages: emptyAs,
			PDQThreshold:  dedup.DefaultPDQThreshold,
			Recognizer:    client,
			SetStatus: func(id string, status session.Status, detail string) {
				u.doc.SetStatus(id, status, detail)
				fyne.Do(func() { u.list.Refresh() })
			},
			SetDuration: func(id string, d time.Duration) {
				u.doc.SetOCRDuration(id, d)
				fyne.Do(func() { u.list.Refresh() })
			},
			OnSlide: func(s markdown.Slide) {
				fyne.Do(func() { u.applySlide(s) })
			},
			OnProgress: func(p pipeline.Progress) {
				fyne.Do(func() {
					u.runProg = p
					if p.Total > 0 {
						u.progress.SetValue(float64(p.Current) / float64(p.Total))
					}
					u.updateRunChrome()
				})
			},
		})
		fyne.Do(func() {
			u.finishRun(res, err)
		})
	}()
}

func (u *App) pause() {
	if u.cancel != nil {
		u.cancel()
	}
}

func (u *App) updateRunChrome() {
	if !u.running {
		return
	}
	elapsed := time.Since(u.runStarted)
	current, total := u.runProg.Current, u.runProg.Total
	if !u.batchRun {
		current, total = 0, 0
	}
	u.setStatus(formatRunLine(u.runProg.Message, elapsed, current, total, false, false))
	if u.list != nil {
		u.list.Refresh()
	}
}

func (u *App) startRunTicker() {
	u.stopRunTicker()
	stop := make(chan struct{})
	u.runTickStop = stop
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				fyne.Do(func() {
					if u.running {
						u.updateRunChrome()
					}
				})
			}
		}
	}()
}

func (u *App) stopRunTicker() {
	if u.runTickStop == nil {
		return
	}
	close(u.runTickStop)
	u.runTickStop = nil
}

func (u *App) finishRun(res pipeline.Result, err error) {
	elapsed := time.Since(u.runStarted)
	u.stopRunTicker()
	u.running = false
	u.batchRun = false
	u.startBtn.Enable()
	u.pauseBtn.Disable()
	u.cancel = nil
	_ = res
	if err != nil && !errors.Is(err, context.Canceled) {
		dialog.ShowError(err, u.win)
		u.setStatus("ошибка: " + err.Error())
		return
	}
	if errors.Is(err, context.Canceled) {
		u.setStatus(formatRunLine("", elapsed, 0, 0, true, false))
		u.list.Refresh()
		return
	}
	u.progress.SetValue(1)
	u.setStatus(formatRunLine(fmt.Sprintf("готово, слайдов в выводе: %d", counted(u.result)), elapsed, 0, 0, false, true))
	if len(u.tabs.Items) > 2 {
		u.tabs.SelectIndex(2)
	}
}

func (u *App) ensureResultSlots(slides []session.Slide) {
	known := make(map[string]struct{}, len(u.result))
	for _, s := range u.result {
		known[s.ID] = struct{}{}
	}
	for _, s := range slides {
		if _, ok := known[s.ID]; ok {
			continue
		}
		u.result = append(u.result, markdown.Slide{ID: s.ID, Name: s.Name})
	}
}

func counted(slides []markdown.Slide) int {
	n := 0
	for _, s := range slides {
		if slideInOutput(s) {
			n++
		}
	}
	return n
}

func (u *App) currentMode() markdown.Mode {
	switch u.outMode.Selected {
	case outSeparator:
		return markdown.ModeSeparator
	case outSeparate:
		return markdown.ModeSeparateFiles
	default:
		return markdown.ModeContinuous
	}
}

func (u *App) saveOutput() {
	if err := u.writeOutput(); err != nil {
		dialog.ShowError(err, u.win)
	}
}

func (u *App) writeOutput() error {
	dir := strings.TrimSpace(u.outDirEntry.Text)
	if dir == "" {
		return errors.New("выберите папку сохранения на вкладке Распознавание")
	}
	slides := append([]markdown.Slide(nil), u.orderedResult()...)
	if counted(slides) == 0 {
		return errors.New("ещё нет готовых слайдов")
	}
	u.savePrefs()
	if err := markdown.SaveToDir(slides, u.currentMode(), dir); err != nil {
		return err
	}
	for _, s := range slides {
		if slideInOutput(s) {
			u.saved[s.ID] = true
		}
	}
	u.setStatus("сохранено в " + dir)
	return nil
}

func (u *App) rerecognize(id string) {
	if u.running {
		dialog.ShowError(errors.New("сначала Пауза"), u.win)
		return
	}
	idx, ok := u.doc.IndexByID(id)
	if !ok {
		return
	}
	s, ok := u.doc.At(idx)
	if !ok {
		return
	}
	u.savePrefs()
	cfg := u.llmConfig()
	if strings.TrimSpace(cfg.Model) == "" {
		dialog.ShowError(errors.New("укажите модель"), u.win)
		return
	}
	promptText := u.promptBox.Text
	if strings.TrimSpace(promptText) == "" {
		u.rebuildPrompt()
		promptText = u.promptBox.Text
	}
	u.running = true
	u.batchRun = false
	u.runStarted = time.Now()
	u.runProg = pipeline.Progress{Message: "распознавание " + s.Name, Total: 1}
	u.startBtn.Disable()
	u.pauseBtn.Enable()
	u.startRunTicker()
	u.updateRunChrome()
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	client := llm.New(cfg)
	illustrations := u.optIllust.Checked
	emptyAs := u.optEmptyIllust.Checked
	go func() {
		md, err := pipeline.RecognizeOne(ctx, pipeline.OneRequest{
			Slide:         s,
			Index:         idx,
			Prompt:        promptText,
			Illustrations: illustrations,
			EmptyAsImages: emptyAs,
			Recognizer:    client,
			SetStatus: func(sid string, status session.Status, detail string) {
				u.doc.SetStatus(sid, status, detail)
				fyne.Do(func() { u.list.Refresh() })
			},
			SetDuration: func(sid string, d time.Duration) {
				u.doc.SetOCRDuration(sid, d)
				fyne.Do(func() { u.list.Refresh() })
			},
		})
		fyne.Do(func() {
			elapsed := time.Since(u.runStarted)
			u.stopRunTicker()
			u.running = false
			u.startBtn.Enable()
			u.pauseBtn.Disable()
			u.cancel = nil
			if err != nil && !errors.Is(err, context.Canceled) {
				dialog.ShowError(err, u.win)
				return
			}
			if errors.Is(err, context.Canceled) {
				u.setStatus(formatRunLine("", elapsed, 0, 0, true, false))
				return
			}
			u.applySlide(md)
			dur := time.Duration(0)
			if sl, ok := u.doc.At(idx); ok {
				dur = sl.OCRDuration
			}
			u.setStatus("заново: " + s.Name + " · " + formatOCRDuration(dur))
		})
	}()
}
