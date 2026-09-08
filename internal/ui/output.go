package ui

import (
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/grafov/slide-ocr/internal/markdown"
)

func (u *App) outputTab() fyne.CanvasObject {
	save := widget.NewButtonWithIcon("Сохранить", theme.DocumentSaveIcon(), u.saveOutput)
	save.Importance = widget.HighImportance
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
	top := container.NewVBox(container.NewHBox(save, u.outToggle), u.outMode)
	return container.NewBorder(top, nil, nil, nil, u.outBody)
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
	text := widget.NewMultiLineEntry()
	text.Wrapping = fyne.TextWrapWord
	text.Scroll = container.ScrollNone
	text.SetText(s.Text)
	text.SetMinRowsVisible(fragmentMinRows(s.Text))
	text.Disable()
	return container.NewBorder(name, nil, nil, nil, text)
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
