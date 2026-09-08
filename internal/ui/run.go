package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	u.doc.ResetRecognition()
	slides := u.doc.Snapshot()
	u.result = make([]markdown.Slide, len(slides))
	for i, s := range slides {
		u.result[i] = markdown.Slide{ID: s.ID, Name: s.Name}
	}
	u.setFragmentsView(true)
	u.refreshList()
	u.running = true
	u.startBtn.Disable()
	u.stopBtn.Enable()
	u.progress.SetValue(0)
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	client := llm.New(cfg)
	illustrations := u.optIllust.Checked
	go func() {
		res, err := pipeline.Run(ctx, pipeline.Request{
			Slides:        slides,
			Prompt:        promptText,
			Illustrations: illustrations,
			PDQThreshold:  dedup.DefaultPDQThreshold,
			Recognizer:    client,
			SetStatus: func(id string, status session.Status, detail string) {
				u.doc.SetStatus(id, status, detail)
				fyne.Do(func() { u.list.Refresh() })
			},
			OnSlide: func(s markdown.Slide) {
				fyne.Do(func() { u.applySlide(s) })
			},
			OnProgress: func(p pipeline.Progress) {
				fyne.Do(func() {
					if p.Total > 0 {
						u.progress.SetValue(float64(p.Current) / float64(p.Total))
					}
					u.setStatus(p.Message)
				})
			},
		})
		fyne.Do(func() {
			u.finishRun(res, err)
		})
	}()
}

func (u *App) stop() {
	if u.cancel != nil {
		u.cancel()
	}
}

func (u *App) finishRun(res pipeline.Result, err error) {
	u.running = false
	u.startBtn.Enable()
	u.stopBtn.Disable()
	u.cancel = nil
	if len(res.Slides) > 0 {
		u.result = res.Slides
		u.syncOutput()
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		dialog.ShowError(err, u.win)
		u.setStatus("ошибка: " + err.Error())
		return
	}
	if errors.Is(err, context.Canceled) {
		u.setStatus("остановлено")
		return
	}
	u.progress.SetValue(1)
	u.setStatus(fmt.Sprintf("готово, слайдов в выводе: %d", counted(u.result)))
	if len(u.tabs.Items) > 2 {
		u.tabs.SelectIndex(2)
	}
}

func counted(slides []markdown.Slide) int {
	n := 0
	for _, s := range slides {
		if !s.Skip && strings.TrimSpace(s.Text) != "" {
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
	dir := strings.TrimSpace(u.outDirEntry.Text)
	if dir == "" {
		dialog.ShowError(errors.New("выберите папку сохранения на вкладке Распознавание"), u.win)
		return
	}
	slides := append([]markdown.Slide(nil), u.result...)
	if counted(slides) == 0 {
		dialog.ShowError(errors.New("ещё нет готовых слайдов"), u.win)
		return
	}
	u.savePrefs()
	if err := markdown.SaveToDir(slides, u.currentMode(), dir); err != nil {
		dialog.ShowError(err, u.win)
		return
	}
	u.setStatus("сохранено в " + dir)
}
