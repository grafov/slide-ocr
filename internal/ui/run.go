package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"

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
	u.refreshList()
	u.running = true
	u.startBtn.Disable()
	u.stopBtn.Enable()
	u.progress.SetValue(0)
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	client := llm.New(cfg)
	slides := u.doc.Snapshot()
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
	if err != nil && !errors.Is(err, context.Canceled) {
		dialog.ShowError(err, u.win)
		u.setStatus("ошибка: " + err.Error())
		return
	}
	u.result = res.Slides
	mode := u.currentMode()
	u.outPreview.SetText(markdown.Stitch(u.result, mode))
	u.progress.SetValue(1)
	if errors.Is(err, context.Canceled) {
		u.setStatus("остановлено")
		return
	}
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
	if len(u.result) == 0 {
		dialog.ShowError(errors.New("сначала запустите распознавание"), u.win)
		return
	}
	u.savePrefs()
	mode := u.currentMode()
	if mode == markdown.ModeSeparateFiles {
		dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, u.win)
				return
			}
			if lu == nil {
				return
			}
			if err = markdown.Save(u.result, mode, lu.Path()); err != nil {
				dialog.ShowError(err, u.win)
				return
			}
			u.setStatus("сохранено в " + lu.Path())
		}, u.win)
		return
	}
	fd := dialog.NewFileSave(func(wc fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if wc == nil {
			return
		}
		path := wc.URI().Path()
		_ = wc.Close()
		if err = markdown.Save(u.result, mode, path); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		u.setStatus("сохранено: " + path)
	}, u.win)
	fd.SetFileName("lecture.md")
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".md"}))
	fd.Show()
}
