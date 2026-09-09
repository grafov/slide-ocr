package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/grafov/slide-ocr/internal/imageutil"
	"github.com/grafov/slide-ocr/internal/session"
)

type previewThumb struct {
	widget.BaseWidget
	img    *canvas.Image
	onDown func()
	onUp   func()
	onSec  func()
}

func newPreviewThumb() *previewThumb {
	p := &previewThumb{}
	p.ExtendBaseWidget(p)
	p.img = canvas.NewImageFromResource(theme.FileImageIcon())
	p.img.SetMinSize(fyne.NewSquareSize(72))
	p.img.FillMode = canvas.ImageFillContain
	return p
}

func (p *previewThumb) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.img)
}

func (p *previewThumb) MouseDown(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonSecondary && p.onDown != nil {
		p.onDown()
	}
}

func (p *previewThumb) MouseUp(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonSecondary && p.onUp != nil {
		p.onUp()
	}
}

func (p *previewThumb) TappedSecondary(*fyne.PointEvent) {
	if p.onSec != nil {
		p.onSec()
	}
}

var (
	_ desktop.Mouseable      = (*previewThumb)(nil)
	_ fyne.SecondaryTappable = (*previewThumb)(nil)
)

type statusCell struct {
	widget.BaseWidget
	label *widget.Label
	check *widget.Button
	empty *widget.Button
	box   *fyne.Container
}

func newStatusCell() *statusCell {
	c := &statusCell{}
	c.ExtendBaseWidget(c)
	c.label = widget.NewLabel("")
	c.label.Truncation = fyne.TextTruncateEllipsis
	c.label.TextStyle = fyne.TextStyle{Italic: true}
	c.check = widget.NewButtonWithIcon("", theme.ConfirmIcon(), nil)
	c.check.Importance = widget.SuccessImportance
	c.check.Hide()
	c.empty = widget.NewButton("Нет текста", nil)
	c.empty.Importance = widget.WarningImportance
	c.empty.Hide()
	c.box = container.NewStack(c.label, c.check, c.empty)
	c.box.Resize(fyne.NewSize(196, 72))
	return c
}

func (c *statusCell) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.box)
}

func (c *statusCell) MinSize() fyne.Size {
	return fyne.NewSize(196, 72)
}

func (c *statusCell) set(s session.Slide, onDone func()) {
	c.empty.Hide()
	c.empty.OnTapped = nil
	if s.Status == session.StatusDone {
		c.label.SetText("")
		c.label.Hide()
		c.check.Show()
		c.check.OnTapped = onDone
		c.check.Enable()
		return
	}
	c.check.OnTapped = nil
	c.check.Hide()
	if s.Status == session.StatusNoText {
		c.label.Hide()
		c.empty.Show()
		c.empty.OnTapped = onDone
		return
	}
	c.label.Show()
	c.label.SetText(statusText(s))
}

type slideRow struct {
	widget.BaseWidget
	thumb   *previewThumb
	name    *widget.Label
	size    *widget.Label
	status  *statusCell
	handle  *dragHandle
	up      *widget.Button
	down    *widget.Button
	include *widget.Check
	box     *fyne.Container
	index   int
	path    string
	onDown  func(int)
	onUp    func()
	onSec   func(int, string)
}

func newSlideRow() *slideRow {
	r := &slideRow{}
	r.ExtendBaseWidget(r)
	r.thumb = newPreviewThumb()
	r.name = widget.NewLabel("name")
	r.name.Truncation = fyne.TextTruncateEllipsis
	r.size = widget.NewLabel("")
	r.size.TextStyle = fyne.TextStyle{Italic: true}
	r.status = newStatusCell()
	r.handle = newDragHandle()
	r.up = widget.NewButtonWithIcon("", theme.MoveUpIcon(), nil)
	r.down = widget.NewButtonWithIcon("", theme.MoveDownIcon(), nil)
	r.include = widget.NewCheck("", nil)
	r.box = container.NewBorder(
		nil, nil, r.thumb,
		container.NewHBox(r.status, r.handle, r.up, r.down, r.include),
		container.NewVBox(r.name, r.size),
	)
	return r
}

func (r *slideRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.box)
}

func (r *slideRow) MouseDown(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonSecondary && r.onDown != nil {
		r.onDown(r.index)
	}
}

func (r *slideRow) MouseUp(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonSecondary && r.onUp != nil {
		r.onUp()
	}
}

func (r *slideRow) TappedSecondary(*fyne.PointEvent) {
	if r.onSec != nil {
		r.onSec(r.index, r.path)
	}
}

var (
	_ desktop.Mouseable      = (*slideRow)(nil)
	_ fyne.SecondaryTappable = (*slideRow)(nil)
)

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
			row.size.SetText(formatSlideMeta(s))
			slideID := s.ID
			row.status.set(s, func() { u.jumpToOutput(slideID) })
			row.path = s.Path
			row.index = id
			row.onDown = u.beginHover
			row.onUp = u.endHover
			row.onSec = u.handleRowSecondary
			row.thumb.onDown = func() { u.beginHover(id) }
			row.thumb.onUp = u.endHover
			row.thumb.onSec = func() { u.handleRowSecondary(id, s.Path) }
			row.include.OnChanged = nil
			row.include.SetChecked(s.Included)
			row.include.OnChanged = func(on bool) { u.setIncluded(slideID, on) }
			if u.running {
				row.include.Disable()
			} else {
				row.include.Enable()
			}
			if s.Thumb != nil {
				row.thumb.img.Image = s.Thumb
				row.thumb.img.Resource = nil
			} else {
				row.thumb.img.Image = nil
				row.thumb.img.Resource = theme.FileImageIcon()
			}
			row.thumb.img.Refresh()
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
				u.onInputOrderChanged()
			}
			row.up.OnTapped = func() {
				u.doc.SwapAdjacent(idx, false)
				u.onInputOrderChanged()
			}
			row.down.OnTapped = func() {
				u.doc.SwapAdjacent(idx, true)
				u.onInputOrderChanged()
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
		return ""
	case session.StatusNoText:
		return "Нет текста"
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

func formatFileSize(n int64) string {
	const (
		kb = 1024
		mb = 1024 * 1024
	)
	switch {
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func (u *App) inputToolbar() fyne.CanvasObject {
	return container.NewHBox(
		widget.NewButtonWithIcon("Загрузить файлы", theme.FileImageIcon(), u.openFiles),
		widget.NewButtonWithIcon("Добавить папку", theme.FolderOpenIcon(), u.openFolder),
		widget.NewButton("Реверс", func() { u.doc.Reverse(); u.onInputOrderChanged() }),
		widget.NewButton("По имени", func() { u.doc.SortByName(); u.onInputOrderChanged() }),
		widget.NewButton("По дате", func() { u.doc.SortByModTime(); u.onInputOrderChanged() }),
	)
}

func (u *App) onInputOrderChanged() {
	u.refreshList()
	u.syncOutput()
}

func (u *App) showImagePreview(path string) {
	go func() {
		img, err := imageutil.OpenFile(path)
		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, u.win) })
			return
		}
		fitted := imageutil.FitForPreview(img)
		fyne.Do(func() {
			view := canvas.NewImageFromImage(fitted)
			view.FillMode = canvas.ImageFillContain
			view.SetMinSize(fyne.NewSize(720, 480))
			d := dialog.NewCustom("Превью", "Закрыть", container.NewScroll(view), u.win)
			d.Resize(fyne.NewSize(900, 700))
			d.Show()
		})
	}()
}

func (u *App) jumpToInput(id string) {
	idx, ok := u.doc.IndexByID(id)
	if !ok {
		return
	}
	u.tabs.SelectIndex(0)
	u.list.Select(idx)
	u.list.ScrollTo(idx)
}

func (u *App) jumpToOutput(id string) {
	u.setFragmentsView(true)
	if len(u.tabs.Items) > 2 {
		u.tabs.SelectIndex(2)
	}
	u.scrollFragmentIntoView(id)
}
