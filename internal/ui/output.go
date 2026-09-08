package ui

import (
	"errors"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/grafov/slide-ocr/internal/markdown"
)

func (u *App) outputTab() fyne.CanvasObject {
	save := widget.NewButtonWithIcon("Сохранить", theme.DocumentSaveIcon(), u.saveOutput)
	save.Importance = widget.HighImportance
	clearBtn := widget.NewButton("Очистить вывод", u.confirmClearOutput)
	u.outToggle = widget.NewButton("Показать весь текст", u.toggleOutView)
	u.outMode.OnChanged = func(_ string) {
		if !u.showFragments {
			u.outPreview.SetText(markdown.Stitch(u.result, u.currentMode()))
		}
	}
	u.fragBox = container.NewVBox()
	u.fragScroll = container.NewVScroll(u.fragBox)
	u.outPreview.Hide()
	u.outBody = container.NewStack(u.fragScroll, u.outPreview)
	top := container.NewVBox(container.NewHBox(save, clearBtn, u.outToggle), u.outMode)
	return container.NewBorder(top, nil, nil, nil, u.outBody)
}

func (u *App) setIncluded(id string, included bool) {
	u.doc.SetIncluded(id, included)
	if !included {
		u.applySlide(markdown.Slide{ID: id, Skip: true})
		delete(u.saved, id)
	}
	if u.list != nil {
		u.list.Refresh()
	}
}

func (u *App) confirmClearOutput() {
	if u.running {
		dialog.ShowError(errors.New("сначала Пауза"), u.win)
		return
	}
	if hasUnsaved(u.result, u.saved) {
		msg := widget.NewLabel("Есть несохранённые фрагменты.")
		box := container.NewVBox(msg)
		d := dialog.NewCustom("Очистить вывод", "Отмена", box, u.win)
		box.Add(container.NewHBox(
			widget.NewButton("Сохранить", func() {
				d.Hide()
				if err := u.writeOutput(); err != nil {
					dialog.ShowError(err, u.win)
					return
				}
				u.clearOutput()
			}),
			widget.NewButton("Сбросить без сохранения", func() {
				d.Hide()
				u.clearOutput()
			}),
		))
		d.Show()
		return
	}
	u.clearOutput()
}

func (u *App) clearOutput() {
	u.doc.ClearOutput()
	u.result = nil
	u.saved = map[string]bool{}
	u.syncOutput()
	u.progress.SetValue(0)
	u.refreshList()
	u.setStatus("вывод очищен")
}

func (u *App) toggleOutView() {
	u.setFragmentsView(!u.showFragments)
}

func (u *App) setFragmentsView(fragments bool) {
	u.showFragments = fragments
	if fragments {
		u.outToggle.SetText("Показать весь текст")
		u.outPreview.Hide()
		u.fragScroll.Show()
		u.rebuildFragments()
	} else {
		u.outToggle.SetText("Показать фрагменты")
		u.fragScroll.Hide()
		u.outPreview.Show()
		u.outPreview.SetText(markdown.Stitch(u.result, u.currentMode()))
	}
	if u.outBody != nil {
		u.outBody.Refresh()
	}
}

func (u *App) applySlide(s markdown.Slide) {
	if idx, ok := u.doc.IndexByID(s.ID); ok {
		if sl, ok := u.doc.At(idx); ok && !sl.Included {
			s.Skip = true
			s.Text = ""
			s.Assets = nil
		}
	}
	found := false
	for i := range u.result {
		if u.result[i].ID == s.ID {
			u.result[i] = s
			found = true
			break
		}
	}
	if !found {
		u.result = append(u.result, s)
	}
	if s.Skip {
		delete(u.saved, s.ID)
	} else if strings.TrimSpace(s.Text) != "" {
		u.saved[s.ID] = false
		u.doc.SetText(s.ID, s.Text)
	}
	u.syncOutput()
	if u.list != nil {
		u.list.Refresh()
	}
}

func (u *App) syncOutput() {
	if u.showFragments {
		off := fyne.NewPos(0, 0)
		if u.fragScroll != nil {
			off = u.fragScroll.Offset
		}
		u.rebuildFragments()
		if u.fragScroll != nil {
			u.fragScroll.ScrollToOffset(off)
		}
		return
	}
	u.outPreview.SetText(markdown.Stitch(u.result, u.currentMode()))
}

func (u *App) rebuildFragments() {
	if u.fragBox == nil {
		return
	}
	u.fragRows = nil
	objs := make([]fyne.CanvasObject, 0)
	for _, s := range u.result {
		if s.Skip || strings.TrimSpace(s.Text) == "" {
			continue
		}
		if idx, ok := u.doc.IndexByID(s.ID); ok {
			if sl, ok := u.doc.At(idx); ok && !sl.Included {
				continue
			}
		}
		row := u.newFragmentRow(s)
		u.fragRows = append(u.fragRows, fragItem{id: s.ID, obj: row})
		objs = append(objs, row)
	}
	u.fragBox.Objects = objs
	u.fragBox.Refresh()
}

func (u *App) newFragmentRow(s markdown.Slide) fyne.CanvasObject {
	id := s.ID
	name := widget.NewHyperlink(s.Name, nil)
	name.OnTapped = func() { u.jumpToInput(id) }
	redo := widget.NewButton("Распознать заново", func() { u.rerecognize(id) })
	head := container.NewBorder(nil, nil, name, redo)
	text := widget.NewMultiLineEntry()
	text.Wrapping = fyne.TextWrapWord
	text.Scroll = container.ScrollNone
	text.SetText(s.Text)
	text.SetMinRowsVisible(fragmentMinRows(s.Text))
	text.Disable()
	return container.NewBorder(head, nil, nil, nil, text)
}

func fragmentMinRows(text string) int {
	n := 0
	for _, line := range strings.Split(text, "\n") {
		cols := utf8.RuneCountInString(line)
		rows := 1
		if cols > 48 {
			rows = (cols + 47) / 48
		}
		n += rows
	}
	if n < 2 {
		return 2
	}
	return n
}

func (u *App) scrollFragmentIntoView(id string) {
	if u.fragScroll == nil {
		return
	}
	var y float32
	pad := theme.Padding()
	for i, row := range u.fragRows {
		if row.id == id {
			u.fragScroll.ScrollToOffset(fyne.NewPos(0, y))
			return
		}
		h := row.obj.MinSize().Height
		if sz := row.obj.Size(); sz.Height > h {
			h = sz.Height
		}
		y += h
		if i < len(u.fragRows)-1 {
			y += pad
		}
	}
}
