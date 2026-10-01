// Package ui builds the three-tab Fyne interface.
package ui

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/grafov/slide-ocr/internal/imageutil"
	"github.com/grafov/slide-ocr/internal/markdown"
	"github.com/grafov/slide-ocr/internal/pipeline"
	"github.com/grafov/slide-ocr/internal/session"
)

const (
	prefURL       = "backend.url"
	prefKey       = "backend.apikey"
	prefModel     = "backend.model"
	prefReason    = "backend.reasoning"
	prefEffort    = "backend.effort"
	prefTemp      = "backend.temperature"
	prefTokens    = "backend.maxtokens"
	prefRU        = "lang.ru"
	prefEN        = "lang.en"
	prefExtra     = "lang.extra"
	prefIllust    = "opt.illustrations"
	prefEmptyAs   = "opt.emptyasimages"
	prefTables    = "opt.tables"
	prefStyles    = "opt.styles"
	prefFrag      = "opt.fragments"
	prefOutMode   = "output.mode"
	prefOutDir    = "output.dir"
	prefExportCmd = "output.exportcmd"
	prefProfiles  = "backend.profiles"

	defaultURL       = "http://127.0.0.1:1234/v1"
	defaultEffort    = effortMedium
	defaultTemp      = "0.2"
	defaultTokens    = "4096"
	defaultExportCmd = `$ pandoc -i %s.md -o %s.pdf --pdf-engine xelatex -V mainfont="Liberation Serif"`

	effortLow    = "low"
	effortMedium = "medium"
	effortHigh   = "high"
)

const (
	outContinuous = "Сплошной текст"
	outSeparator  = "Разделитель -----"
	outSeparate   = "Отдельные файлы"
)

type fragItem struct {
	id  string
	obj fyne.CanvasObject
}

// App is the main window content and actions.
type App struct {
	fyneApp fyne.App
	win     fyne.Window
	doc     *session.Document
	root    fyne.CanvasObject

	list        *widget.List
	status      *widget.Label
	progress    *widget.ProgressBar
	startBtn    *widget.Button
	pauseBtn    *widget.Button
	promptBox   *widget.Entry
	promptDirty bool
	rebuilding  bool

	profiles    profileSet
	profileBox  *fyne.Container
	backendForm fyne.CanvasObject
	nameEntry   *widget.Entry
	activeCheck *widget.Check
	profileLock bool

	urlEntry       *widget.Entry
	keyEntry       *widget.Entry
	modelEntry     *widget.Entry
	modelSelect    *widget.Select
	reasonChk      *widget.Check
	effortSel      *widget.Select
	tempEntry      *widget.Entry
	tokensEntry    *widget.Entry
	langRU         *widget.Check
	langEN         *widget.Check
	langExtra      *widget.Entry
	optIllust      *widget.Check
	optEmptyIllust *widget.Check
	optTables      *widget.Check
	optStyles      *widget.Check
	optFrag        *widget.Check
	outDirEntry    *widget.Entry
	exportEntry    *widget.Entry
	exportBtn      *widget.Button

	outMode       *widget.RadioGroup
	outPreview    *widget.Entry
	outToggle     *widget.Button
	fragScroll    *container.Scroll
	fragBox       *fyne.Container
	fragRows      []fragItem
	showFragments bool
	outBody       *fyne.Container

	tabs        *container.AppTabs
	running     bool
	batchRun    bool
	cancel      context.CancelFunc
	runStarted  time.Time
	runProg     pipeline.Progress
	runTickStop chan struct{}
	result      []markdown.Slide
	saved       map[string]bool
	hoverHeld   bool
	hoverStart  int
	hoverLast   int
	hoverCard   fyne.CanvasObject
	hoverLayer  *hoverLayer
	hoverImg    *canvas.Image
	hoverLabel  *widget.Label
	hoverGen    atomic.Uint64
}

// New constructs the tabbed UI.
func New(a fyne.App, win fyne.Window) *App {
	u := &App{
		fyneApp:       a,
		win:           win,
		doc:           &session.Document{},
		status:        widget.NewLabel("Добавьте изображения слайдов."),
		progress:      widget.NewProgressBar(),
		showFragments: true,
		saved:         map[string]bool{},
		hoverStart:    -1,
		hoverLast:     -1,
	}
	u.status.Truncation = fyne.TextTruncateEllipsis
	u.progress.Min = 0
	u.progress.Max = 1
	u.buildRecognizeControls()
	u.list = u.buildList()
	u.list.OnHighlighted = u.onListHighlighted
	u.buildHoverCard()
	u.outPreview = widget.NewMultiLineEntry()
	u.outPreview.Wrapping = fyne.TextWrapWord
	u.outMode = widget.NewRadioGroup([]string{outContinuous, outSeparator, outSeparate}, nil)
	u.outMode.Required = true
	savedMode := a.Preferences().StringWithFallback(prefOutMode, outContinuous)
	u.outMode.SetSelected(savedMode)

	inputTab := container.NewBorder(u.inputToolbar(), nil, nil, nil, u.list)
	recTab := u.recognizeTab()
	outTab := u.outputTab()

	u.tabs = container.NewAppTabs(
		container.NewTabItem("Вход", inputTab),
		container.NewTabItem("Распознавание", recTab),
		container.NewTabItem("Вывод", outTab),
	)
	chrome := container.NewVBox(u.progress, u.status)
	main := container.NewBorder(nil, chrome, nil, nil, u.tabs)
	u.root = container.NewStack(main, u.hoverLayer)
	win.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		paths := make([]string, 0, len(uris))
		for _, uri := range uris {
			paths = append(paths, uri.Path())
		}
		u.addPaths(paths)
	})
	u.loadPrefs()
	u.rebuildPrompt()
	return u
}

// Canvas is the root widget for the window.
func (u *App) Canvas() fyne.CanvasObject {
	return u.root
}

func (u *App) setStatus(s string) {
	u.status.SetText(s)
}

func (u *App) refreshList() {
	if u.list != nil {
		u.list.Refresh()
	}
	if u.running {
		return
	}
	n := u.doc.Len()
	u.setStatus(fmt.Sprintf("Файлов: %d", n))
}

func (u *App) addPaths(paths []string) {
	collected, err := imageutil.CollectImages(paths)
	if err != nil {
		dialog.ShowError(err, u.win)
		return
	}
	added := u.doc.AddPaths(collected)
	u.refreshList()
	if added == 0 {
		return
	}
	go func() {
		u.doc.LoadThumbs(context.Background(), func() {
			fyne.Do(func() { u.list.Refresh() })
		})
		fyne.Do(func() { u.refreshList() })
	}()
}

func (u *App) openFiles() {
	fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if rc == nil {
			return
		}
		path := rc.URI().Path()
		_ = rc.Close()
		u.addPaths([]string{path})
	}, u.win)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{
		".jpg", ".jpeg", ".png", ".webp", ".tif", ".tiff", ".gif", ".bmp",
	}))
	fd.Show()
}

func (u *App) openFolder() {
	dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if lu == nil {
			return
		}
		u.addPaths([]string{lu.Path()})
	}, u.win)
}

func (u *App) pickSaveDir() {
	dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if lu == nil {
			return
		}
		u.outDirEntry.SetText(lu.Path())
		u.savePrefs()
	}, u.win)
}
