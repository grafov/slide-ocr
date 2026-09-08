package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/grafov/slide-ocr/internal/session"
)

type slideRow struct {
	widget.BaseWidget
	thumb  *canvas.Image
	name   *widget.Label
	status *widget.Label
	handle *dragHandle
	up     *widget.Button
	down   *widget.Button
	box    *fyne.Container
}

func newSlideRow() *slideRow {
	r := &slideRow{}
	r.ExtendBaseWidget(r)
	r.thumb = canvas.NewImageFromResource(theme.FileImageIcon())
	r.thumb.SetMinSize(fyne.NewSquareSize(72))
	r.thumb.FillMode = canvas.ImageFillContain
	r.name = widget.NewLabel("name")
	r.name.Truncation = fyne.TextTruncateEllipsis
	r.status = widget.NewLabel("")
	r.status.TextStyle = fyne.TextStyle{Italic: true}
	r.handle = newDragHandle()
	r.up = widget.NewButtonWithIcon("", theme.MoveUpIcon(), nil)
	r.down = widget.NewButtonWithIcon("", theme.MoveDownIcon(), nil)
	r.box = container.NewBorder(
		nil, nil, r.thumb,
		container.NewHBox(r.handle, r.up, r.down),
		container.NewVBox(r.name, r.status),
	)
	return r
}

func (r *slideRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.box)
}

func (u *App) buildList() *widget.List {
	return widget.NewList(
		func() int { return u.doc.Len() },
		func() fyne.CanvasObject { return newSlideRow() },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row := obj.(*slideRow)
			s, ok := u.doc.At(id)
			if !ok {
				return
			}
			row.name.SetText(s.Name)
			row.status.SetText(statusText(s))
			if s.Thumb != nil {
				row.thumb.Image = s.Thumb
				row.thumb.Resource = nil
			} else {
				row.thumb.Image = nil
				row.thumb.Resource = theme.FileImageIcon()
			}
			row.thumb.Refresh()
			idx := id
			row.handle.index = idx
			row.handle.onDrop = func(from, delta int) {
				to := from + delta
				n := u.doc.Len()
				if to < 0 {
					to = 0
				}
				if to >= n {
					to = n - 1
				}
				u.doc.Move(from, to)
				u.refreshList()
			}
			row.up.OnTapped = func() {
				u.doc.SwapAdjacent(idx, false)
				u.refreshList()
			}
			row.down.OnTapped = func() {
				u.doc.SwapAdjacent(idx, true)
				u.refreshList()
			}
			row.up.Enable()
			row.down.Enable()
			if idx == 0 {
				row.up.Disable()
			}
			if idx == u.doc.Len()-1 {
				row.down.Disable()
			}
		},
	)
}

func statusText(s session.Slide) string {
	switch s.Status {
	case session.StatusRunning:
		return "распознавание…"
	case session.StatusDone:
		return "готово"
	case session.StatusSkipped:
		if s.Detail != "" {
			return s.Detail
		}
		return "пропуск"
	case session.StatusError:
		if s.Detail != "" {
			return "ошибка: " + s.Detail
		}
		return "ошибка"
	default:
		return ""
	}
}

func (u *App) inputToolbar() fyne.CanvasObject {
	return container.NewHBox(
		widget.NewButtonWithIcon("Загрузить файлы", theme.FileImageIcon(), u.openFiles),
		widget.NewButtonWithIcon("Добавить папку", theme.FolderOpenIcon(), u.openFolder),
		widget.NewButton("Реверс", func() { u.doc.Reverse(); u.refreshList() }),
		widget.NewButton("По имени", func() { u.doc.SortByName(); u.refreshList() }),
		widget.NewButton("По дате", func() { u.doc.SortByModTime(); u.refreshList() }),
	)
}
