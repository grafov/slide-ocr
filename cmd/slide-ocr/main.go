package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/grafov/slide-ocr/internal/ui"
)

var (
	version   = "dev"
	gitCommit = "unknown"
)

func main() {
	a := app.NewWithID("com.grafov.slideocr")
	w := a.NewWindow("slide-ocr " + version + " (" + gitCommit + ")")
	w.Resize(fyne.NewSize(1100, 760))
	uiApp := ui.New(a, w)
	w.SetContent(uiApp.Canvas())
	w.ShowAndRun()
}
