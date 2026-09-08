package ui

import (
	"context"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/grafov/slide-ocr/internal/llm"
	"github.com/grafov/slide-ocr/internal/prompt"
)

func (u *App) buildRecognizeControls() {
	u.urlEntry = widget.NewEntry()
	u.urlEntry.SetPlaceHolder(defaultURL)
	u.keyEntry = widget.NewPasswordEntry()
	u.keyEntry.SetPlaceHolder("пусто = lm-studio")
	u.modelEntry = widget.NewEntry()
	u.modelEntry.SetPlaceHolder("id модели")
	u.modelSelect = widget.NewSelect(nil, func(v string) {
		u.modelEntry.SetText(v)
	})
	u.reasonChk = widget.NewCheck("Reasoning", u.onPromptOptions)
	u.effortSel = widget.NewSelect([]string{"low", "medium", "high"}, nil)
	u.effortSel.SetSelected("medium")
	u.tempEntry = widget.NewEntry()
	u.tempEntry.SetText("0.2")
	u.tokensEntry = widget.NewEntry()
	u.tokensEntry.SetText("4096")
	u.langRU = widget.NewCheck("русский", u.onPromptOptions)
	u.langEN = widget.NewCheck("английский", u.onPromptOptions)
	u.langExtra = widget.NewEntry()
	u.langExtra.SetPlaceHolder("другие языки через запятую")
	u.langExtra.OnChanged = func(string) { u.onPromptOptions(false) }
	u.optIllust = widget.NewCheck("Включать иллюстрации", u.onPromptOptions)
	u.optTables = widget.NewCheck("Распознавать таблицы", u.onPromptOptions)
	u.optStyles = widget.NewCheck("Выделение стилей", u.onPromptOptions)
	u.optFrag = widget.NewCheck("Дополнять обрывки", u.onPromptOptions)

	u.promptBox = widget.NewMultiLineEntry()
	u.promptBox.Wrapping = fyne.TextWrapWord
	u.promptBox.SetMinRowsVisible(8)
	u.promptBox.OnChanged = func(string) {
		if u.rebuilding {
			return
		}
		u.promptDirty = true
	}

	u.startBtn = widget.NewButtonWithIcon("Пуск", theme.MediaPlayIcon(), u.start)
	u.startBtn.Importance = widget.HighImportance
	u.stopBtn = widget.NewButtonWithIcon("Стоп", theme.MediaStopIcon(), u.stop)
	u.stopBtn.Disable()
}

func (u *App) recognizeTab() fyne.CanvasObject {
	refresh := widget.NewButtonWithIcon("Модели", theme.ViewRefreshIcon(), u.refreshModels)
	rebuild := widget.NewButton("Собрать промпт заново", func() {
		u.promptDirty = false
		u.rebuildPrompt()
	})
	top := container.NewHBox(
		widget.NewButtonWithIcon("Загрузить файлы", theme.FileImageIcon(), u.openFiles),
		widget.NewButtonWithIcon("Добавить папку", theme.FolderOpenIcon(), u.openFolder),
		u.startBtn,
		u.stopBtn,
	)
	form := widget.NewForm(
		widget.NewFormItem("URL", u.urlEntry),
		widget.NewFormItem("API key", u.keyEntry),
		widget.NewFormItem("Модель", container.NewBorder(nil, nil, nil, refresh, container.NewVBox(u.modelSelect, u.modelEntry))),
		widget.NewFormItem("Reasoning", container.NewHBox(u.reasonChk, widget.NewLabel("effort"), u.effortSel)),
		widget.NewFormItem("Temperature", u.tempEntry),
		widget.NewFormItem("Max tokens", u.tokensEntry),
		widget.NewFormItem("Языки", container.NewVBox(container.NewHBox(u.langRU, u.langEN), u.langExtra)),
		widget.NewFormItem("Опции", container.NewVBox(u.optIllust, u.optTables, u.optStyles, u.optFrag)),
	)
	mid := container.NewVScroll(container.NewVBox(form, rebuild, widget.NewLabel("Итоговый промпт")))
	bottom := container.NewBorder(u.progress, nil, nil, nil, u.promptBox)
	return container.NewBorder(top, bottom, nil, nil, mid)
}

func (u *App) onPromptOptions(_ bool) {
	if !u.promptDirty {
		u.rebuildPrompt()
	}
}

func (u *App) promptOptions() prompt.Options {
	return prompt.Options{
		Russian:           u.langRU.Checked,
		English:           u.langEN.Checked,
		ExtraLanguages:    u.langExtra.Text,
		Illustrations:     u.optIllust.Checked,
		Tables:            u.optTables.Checked,
		Styles:            u.optStyles.Checked,
		CompleteFragments: u.optFrag.Checked,
	}
}

func (u *App) rebuildPrompt() {
	u.rebuilding = true
	u.promptBox.SetText(prompt.Build(u.promptOptions()))
	u.rebuilding = false
}

func (u *App) llmConfig() llm.Config {
	temp := float32(0.2)
	if v, err := strconv.ParseFloat(strings.TrimSpace(u.tempEntry.Text), 32); err == nil {
		temp = float32(v)
	}
	tokens := 4096
	if v, err := strconv.Atoi(strings.TrimSpace(u.tokensEntry.Text)); err == nil {
		tokens = v
	}
	return llm.Config{
		BaseURL:         strings.TrimSpace(u.urlEntry.Text),
		APIKey:          strings.TrimSpace(u.keyEntry.Text),
		Model:           strings.TrimSpace(u.modelEntry.Text),
		Reasoning:       u.reasonChk.Checked,
		ReasoningEffort: u.effortSel.Selected,
		Temperature:     temp,
		MaxTokens:       tokens,
	}
}

func (u *App) refreshModels() {
	cfg := u.llmConfig()
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultURL
		u.urlEntry.SetText(cfg.BaseURL)
	}
	client := llm.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	go func() {
		defer cancel()
		names, err := client.ListModels(ctx)
		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(err, u.win)
				return
			}
			u.modelSelect.SetOptions(names)
			if u.modelEntry.Text == "" && len(names) > 0 {
				u.modelSelect.SetSelected(names[0])
			}
		})
	}()
}

func (u *App) loadPrefs() {
	p := u.fyneApp.Preferences()
	u.urlEntry.SetText(p.StringWithFallback(prefURL, defaultURL))
	u.keyEntry.SetText(p.String(prefKey))
	u.modelEntry.SetText(p.String(prefModel))
	u.reasonChk.SetChecked(p.BoolWithFallback(prefReason, false))
	u.effortSel.SetSelected(p.StringWithFallback(prefEffort, "medium"))
	u.tempEntry.SetText(p.StringWithFallback(prefTemp, "0.2"))
	u.tokensEntry.SetText(p.StringWithFallback(prefTokens, "4096"))
	u.langRU.SetChecked(p.BoolWithFallback(prefRU, true))
	u.langEN.SetChecked(p.BoolWithFallback(prefEN, true))
	u.langExtra.SetText(p.String(prefExtra))
	u.optIllust.SetChecked(p.BoolWithFallback(prefIllust, false))
	u.optTables.SetChecked(p.BoolWithFallback(prefTables, true))
	u.optStyles.SetChecked(p.BoolWithFallback(prefStyles, true))
	u.optFrag.SetChecked(p.BoolWithFallback(prefFrag, false))
}

func (u *App) savePrefs() {
	p := u.fyneApp.Preferences()
	p.SetString(prefURL, u.urlEntry.Text)
	p.SetString(prefKey, u.keyEntry.Text)
	p.SetString(prefModel, u.modelEntry.Text)
	p.SetBool(prefReason, u.reasonChk.Checked)
	p.SetString(prefEffort, u.effortSel.Selected)
	p.SetString(prefTemp, u.tempEntry.Text)
	p.SetString(prefTokens, u.tokensEntry.Text)
	p.SetBool(prefRU, u.langRU.Checked)
	p.SetBool(prefEN, u.langEN.Checked)
	p.SetString(prefExtra, u.langExtra.Text)
	p.SetBool(prefIllust, u.optIllust.Checked)
	p.SetBool(prefTables, u.optTables.Checked)
	p.SetBool(prefStyles, u.optStyles.Checked)
	p.SetBool(prefFrag, u.optFrag.Checked)
	p.SetString(prefOutMode, u.outMode.Selected)
}
