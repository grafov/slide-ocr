package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type dragHandle struct {
	widget.Icon
	index  int
	acc    float32
	onDrop func(from, delta int)
}

func newDragHandle() *dragHandle {
	h := &dragHandle{}
	h.ExtendBaseWidget(h)
	h.Resource = theme.ListIcon()
	return h
}

func (h *dragHandle) Dragged(e *fyne.DragEvent) {
	h.acc += e.Dragged.DY
}

func (h *dragHandle) DragEnd() {
	const rowH float32 = 80
	delta := int(h.acc / rowH)
	h.acc = 0
	if delta != 0 && h.onDrop != nil {
		h.onDrop(h.index, delta)
	}
}

var _ fyne.Draggable = (*dragHandle)(nil)
