package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/grafov/slide-ocr/internal/markdown"
)

func (u *App) outputTab() fyne.CanvasObject {
	save := widget.NewButtonWithIcon("Сохранить", theme.DocumentSaveIcon(), u.saveOutput)
	save.Importance = widget.HighImportance
	u.outMode.OnChanged = func(_ string) {
		if len(u.result) > 0 {
			u.outPreview.SetText(markdown.Stitch(u.result, u.currentMode()))
		}
	}
	top := container.NewVBox(save, u.outMode)
	return container.NewBorder(top, nil, nil, nil, u.outPreview)
}
