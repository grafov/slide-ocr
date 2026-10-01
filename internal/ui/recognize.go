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
		if u.profileLock || v == "" {
			return
		}
		u.modelEntry.SetText(v)
	})
	u.reasonChk = widget.NewCheck("Reasoning", u.onPromptOptions)
	u.effortSel = widget.NewSelect([]string{effortLow, effortMedium, effortHigh}, nil)
	u.effortSel.SetSelected(defaultEffort)
	u.tempEntry = widget.NewEntry()
	u.tempEntry.SetText(defaultTemp)
	u.tokensEntry = widget.NewEntry()
	u.tokensEntry.SetText(defaultTokens)
	u.nameEntry = widget.NewEntry()
	u.nameEntry.SetPlaceHolder("имя профиля")
	u.nameEntry.OnChanged = func(v string) {
		if u.profileLock {
			return
		}
		idx := profileIndex(u.profiles, u.profiles.Active)
		if idx < 0 {
			return
		}
		u.profiles.Items[idx].Name = v
		if u.activeCheck != nil {
			u.activeCheck.Text = profileLabel(v)
			u.activeCheck.Refresh()
		}
	}
	u.langRU = widget.NewCheck("русский", u.onPromptOptions)
	u.langEN = widget.NewCheck("английский", u.onPromptOptions)
	u.langExtra = widget.NewEntry()
	u.langExtra.SetPlaceHolder("другие языки через запятую")
	u.langExtra.OnChanged = func(string) { u.onPromptOptions(false) }
	u.optIllust = widget.NewCheck("Включать иллюстрации", u.onPromptOptions)
	u.optEmptyIllust = widget.NewCheck("Вставлять пустые слайды как иллюстрации", nil)
	u.optTables = widget.NewCheck("Распознавать таблицы", u.onPromptOptions)
	u.optStyles = widget.NewCheck("Выделение стилей", u.onPromptOptions)
	u.optFrag = widget.NewCheck("Дополнять обрывки", u.onPromptOptions)
	u.outDirEntry = widget.NewEntry()
	u.outDirEntry.SetPlaceHolder("папка для Markdown")

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
	u.pauseBtn = widget.NewButtonWithIcon("Пауза", theme.MediaPauseIcon(), u.pause)
	u.pauseBtn.Disable()
}

func (u *App) recognizeTab() fyne.CanvasObject {
	refresh := widget.NewButtonWithIcon("Модели", theme.ViewRefreshIcon(), u.refreshModels)
	rebuild := widget.NewButton("Собрать промпт заново", func() {
		u.promptDirty = false
		u.rebuildPrompt()
	})
	top := container.NewHBox(
		u.startBtn,
		u.pauseBtn,
	)
	browseDir := widget.NewButtonWithIcon("Выбрать папку…", theme.FolderOpenIcon(), u.pickSaveDir)
	u.backendForm = widget.NewForm(
		widget.NewFormItem("Имя", u.nameEntry),
		widget.NewFormItem("URL", u.urlEntry),
		widget.NewFormItem("API key", u.keyEntry),
		widget.NewFormItem("Модель", container.NewBorder(nil, nil, nil, refresh, container.NewVBox(u.modelSelect, u.modelEntry))),
		widget.NewFormItem("Reasoning", container.NewHBox(u.reasonChk, widget.NewLabel("effort"), u.effortSel)),
		widget.NewFormItem("Temperature", u.tempEntry),
		widget.NewFormItem("Max tokens", u.tokensEntry),
	)
	shared := widget.NewForm(
		widget.NewFormItem("Языки", container.NewVBox(container.NewHBox(u.langRU, u.langEN), u.langExtra)),
		widget.NewFormItem("Опции", container.NewVBox(u.optIllust, u.optEmptyIllust, u.optTables, u.optStyles, u.optFrag)),
		widget.NewFormItem("Папка сохранения", container.NewBorder(nil, nil, nil, browseDir, u.outDirEntry)),
	)
	addBtn := widget.NewButton("Добавить профиль", u.addProfile)
	u.profileBox = container.NewVBox()
	mid := container.NewVScroll(container.NewVBox(addBtn, u.profileBox, shared, rebuild, widget.NewLabel("Итоговый промпт")))
	return container.NewBorder(top, u.promptBox, nil, nil, mid)
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
	active := u.profiles.Active
	client := llm.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	go func() {
		defer cancel()
		names, err := client.ListModels(ctx)
		fyne.Do(func() {
			if u.profiles.Active != active {
				return
			}
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
	u.profiles = profilesFromStored(p.String(prefProfiles), legacyBackend{
		URL:       p.StringWithFallback(prefURL, defaultURL),
		APIKey:    p.String(prefKey),
		Model:     p.String(prefModel),
		Reasoning: p.BoolWithFallback(prefReason, false),
		Effort:    p.StringWithFallback(prefEffort, defaultEffort),
		Temp:      p.StringWithFallback(prefTemp, defaultTemp),
		Tokens:    p.StringWithFallback(prefTokens, defaultTokens),
	})
	u.loadActiveWidgets()
	u.renderProfiles()
	u.langRU.SetChecked(p.BoolWithFallback(prefRU, true))
	u.langEN.SetChecked(p.BoolWithFallback(prefEN, true))
	u.langExtra.SetText(p.String(prefExtra))
	u.optIllust.SetChecked(p.BoolWithFallback(prefIllust, false))
	u.optEmptyIllust.SetChecked(p.BoolWithFallback(prefEmptyAs, false))
	u.optTables.SetChecked(p.BoolWithFallback(prefTables, true))
	u.optStyles.SetChecked(p.BoolWithFallback(prefStyles, true))
	u.optFrag.SetChecked(p.BoolWithFallback(prefFrag, false))
	u.outDirEntry.SetText(p.String(prefOutDir))
}

func (u *App) savePrefs() {
	u.flushActive()
	p := u.fyneApp.Preferences()
	if raw, err := marshalProfiles(u.profiles); err == nil {
		p.SetString(prefProfiles, raw)
	}
	p.SetBool(prefRU, u.langRU.Checked)
	p.SetBool(prefEN, u.langEN.Checked)
	p.SetString(prefExtra, u.langExtra.Text)
	p.SetBool(prefIllust, u.optIllust.Checked)
	p.SetBool(prefEmptyAs, u.optEmptyIllust.Checked)
	p.SetBool(prefTables, u.optTables.Checked)
	p.SetBool(prefStyles, u.optStyles.Checked)
	p.SetBool(prefFrag, u.optFrag.Checked)
	p.SetString(prefOutMode, u.outMode.Selected)
	p.SetString(prefOutDir, u.outDirEntry.Text)
}

func (u *App) flushActive() {
	idx := profileIndex(u.profiles, u.profiles.Active)
	if idx < 0 || u.nameEntry == nil {
		return
	}
	p := u.profiles.Items[idx]
	p.Name = u.nameEntry.Text
	p.URL = u.urlEntry.Text
	p.APIKey = u.keyEntry.Text
	p.Model = u.modelEntry.Text
	p.Reasoning = u.reasonChk.Checked
	p.Effort = u.effortSel.Selected
	p.Temp = u.tempEntry.Text
	p.Tokens = u.tokensEntry.Text
	u.profiles.Items[idx] = p
}

func (u *App) loadActiveWidgets() {
	idx := profileIndex(u.profiles, u.profiles.Active)
	if idx < 0 {
		return
	}
	p := u.profiles.Items[idx]
	u.profileLock = true
	u.nameEntry.SetText(p.Name)
	u.urlEntry.SetText(p.URL)
	u.keyEntry.SetText(p.APIKey)
	u.modelSelect.SetOptions(nil)
	u.modelSelect.ClearSelected()
	u.modelEntry.SetText(p.Model)
	u.reasonChk.SetChecked(p.Reasoning)
	u.effortSel.SetSelected(p.Effort)
	u.tempEntry.SetText(p.Temp)
	u.tokensEntry.SetText(p.Tokens)
	u.profileLock = false
}

func (u *App) renderProfiles() {
	if u.profileBox == nil {
		return
	}
	u.activeCheck = nil
	u.profileLock = true
	canDelete := len(u.profiles.Items) > 1
	rows := make([]fyne.CanvasObject, 0, len(u.profiles.Items)*2)
	for _, item := range u.profiles.Items {
		rows = append(rows, u.profileHeader(item, canDelete))
		if item.ID == u.profiles.Active && u.backendForm != nil {
			rows = append(rows, u.backendForm)
		}
	}
	u.profileBox.Objects = rows
	u.profileBox.Refresh()
	u.profileLock = false
}

func (u *App) profileHeader(item backendProfile, canDelete bool) fyne.CanvasObject {
	id := item.ID
	chk := widget.NewCheck(profileLabel(item.Name), nil)
	chk.OnChanged = func(on bool) {
		u.onProfileChecked(id, chk, on)
	}
	if id == u.profiles.Active {
		chk.SetChecked(true)
		u.activeCheck = chk
	}
	if !canDelete {
		return chk
	}
	del := widget.NewButton("Удалить", func() {
		u.confirmDeleteProfile(id)
	})
	del.Importance = widget.LowImportance

	return container.NewBorder(nil, nil, nil, del, chk)
}

func (u *App) onProfileChecked(id string, chk *widget.Check, on bool) {
	if u.profileLock {
		return
	}
	if !on {
		u.profileLock = true
		chk.SetChecked(true)
		u.profileLock = false

		return
	}
	u.activateProfile(id)
}

func (u *App) activateProfile(id string) {
	if id == "" || id == u.profiles.Active {
		return
	}
	u.flushActive()
	next, err := selectProfile(u.profiles, id)
	if err != nil {
		u.renderProfiles()
		return
	}
	u.profiles = next
	u.loadActiveWidgets()
	u.renderProfiles()
	u.savePrefs()
}

func (u *App) addProfile() {
	u.flushActive()
	u.profiles = appendProfile(u.profiles)
	u.loadActiveWidgets()
	u.renderProfiles()
	u.savePrefs()
}

func (u *App) confirmDeleteProfile(id string) {
	if len(u.profiles.Items) <= 1 {
		return
	}
	idx := profileIndex(u.profiles, id)
	if idx < 0 {
		return
	}
	title := profileLabel(u.profiles.Items[idx].Name)
	dialog.ShowConfirm("Удалить профиль", "Удалить профиль «"+title+"»?", func(ok bool) {
		if !ok {
			return
		}
		u.deleteProfile(id)
	}, u.win)
}

func (u *App) deleteProfile(id string) {
	u.flushActive()
	prev := u.profiles.Active
	next, err := removeProfile(u.profiles, id)
	if err != nil {
		return
	}
	u.profiles = next
	if u.profiles.Active != prev {
		u.loadActiveWidgets()
	}
	u.renderProfiles()
	u.savePrefs()
}
