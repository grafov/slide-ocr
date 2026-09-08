package ui

import (
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/grafov/slide-ocr/internal/imageutil"
	"github.com/grafov/slide-ocr/internal/session"
)

type hoverCatcher struct {
	widget.BaseWidget
	inner fyne.CanvasObject
	onUp  func()
}

func newHoverCatcher(inner fyne.CanvasObject, onUp func()) *hoverCatcher {
	c := &hoverCatcher{inner: inner, onUp: onUp}
	c.ExtendBaseWidget(c)
	return c
}

func (c *hoverCatcher) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.inner)
}

func (c *hoverCatcher) MouseDown(*desktop.MouseEvent) {}

func (c *hoverCatcher) MouseUp(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonSecondary && c.onUp != nil {
		c.onUp()
	}
}

var _ desktop.Mouseable = (*hoverCatcher)(nil)

type hoverLayer struct {
	widget.BaseWidget
	card fyne.CanvasObject
}

func newHoverLayer(card fyne.CanvasObject) *hoverLayer {
	l := &hoverLayer{card: card}
	l.ExtendBaseWidget(l)
	return l
}

func (l *hoverLayer) CreateRenderer() fyne.WidgetRenderer {
	return &hoverLayerRenderer{layer: l, objs: []fyne.CanvasObject{l.card}}
}

type hoverLayerRenderer struct {
	layer *hoverLayer
	objs  []fyne.CanvasObject
}

func (r *hoverLayerRenderer) Layout(size fyne.Size) {
	ms := fyne.NewSize(360, 240)
	if cmin := r.layer.card.MinSize(); cmin.Width > ms.Width {
		ms.Width = cmin.Width
	}
	if cmin := r.layer.card.MinSize(); cmin.Height > ms.Height {
		ms.Height = cmin.Height
	}
	pad := float32(12)
	x := size.Width - ms.Width - pad
	if x < pad {
		x = pad
	}
	r.layer.card.Resize(ms)
	r.layer.card.Move(fyne.NewPos(x, pad))
}

func (r *hoverLayerRenderer) MinSize() fyne.Size { return fyne.NewSize(0, 0) }

func (r *hoverLayerRenderer) Objects() []fyne.CanvasObject { return r.objs }

func (r *hoverLayerRenderer) Refresh() {}

func (r *hoverLayerRenderer) Destroy() {}

func (u *App) buildHoverCard() {
	u.hoverImg = canvas.NewImageFromResource(theme.FileImageIcon())
	u.hoverImg.FillMode = canvas.ImageFillContain
	u.hoverImg.SetMinSize(fyne.NewSize(360, 240))
	u.hoverLabel = widget.NewLabel("")
	u.hoverLabel.Truncation = fyne.TextTruncateEllipsis
	inner := container.NewBorder(u.hoverLabel, nil, nil, nil, u.hoverImg)
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameOverlayBackground))
	card := container.NewStack(bg, container.NewPadded(inner))
	u.hoverCard = newHoverCatcher(card, u.endHover)
	u.hoverCard.Hide()
	u.hoverLayer = newHoverLayer(u.hoverCard)
}

func (u *App) onListHighlighted(id widget.ListItemID) {
	if !u.hoverHeld {
		return
	}
	u.hoverLast = id
	s, ok := u.doc.At(id)
	if !ok {
		return
	}
	u.showHover(s)
}

func (u *App) beginHover(idx int) {
	s, ok := u.doc.At(idx)
	if !ok {
		return
	}
	u.hoverHeld = true
	u.hoverStart = idx
	u.hoverLast = idx
	u.showHover(s)
}

func (u *App) endHover() {
	u.hoverHeld = false
	if u.hoverCard == nil {
		return
	}
	u.hoverCard.Hide()
	if u.hoverLayer != nil {
		u.hoverLayer.Refresh()
	}
}

func (u *App) showHover(s session.Slide) {
	if u.hoverCard == nil {
		return
	}
	u.hoverLabel.SetText(s.Name)
	if s.Thumb != nil {
		u.hoverImg.Image = s.Thumb
		u.hoverImg.Resource = nil
		u.hoverImg.Refresh()
	}
	u.hoverCard.Show()
	if u.hoverLayer != nil {
		u.hoverLayer.Refresh()
	}
	u.hoverCard.Refresh()
	gen := u.hoverGen.Add(1)
	path := s.Path
	go func() {
		img, err := imageutil.OpenFile(path)
		if err != nil {
			return
		}
		fitted := imageutil.FitForHover(img)
		fyne.Do(func() {
			if u.hoverGen.Load() != gen || !u.hoverHeld {
				return
			}
			u.setHoverImage(fitted)
		})
	}()
}

func (u *App) setHoverImage(img image.Image) {
	u.hoverImg.Image = img
	u.hoverImg.Resource = nil
	u.hoverImg.Refresh()
}

func (u *App) handleRowSecondary(idx int, path string) {
	if shouldOpenDialog(u.hoverStart, u.hoverLast) {
		u.showImagePreview(path)
	}
	u.hoverStart = -1
	u.hoverLast = -1
}
