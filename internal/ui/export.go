package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"github.com/grafov/slide-ocr/internal/markdown"
)

func (u *App) exportOutput() {
	dir := strings.TrimSpace(u.outDirEntry.Text)
	if dir == "" {
		dialog.ShowError(errors.New("выберите папку сохранения"), u.win)
		return
	}
	tmpl := u.exportEntry.Text
	stems := markdown.MarkdownStems(u.orderedResult(), u.currentMode())
	if len(stems) == 0 {
		dialog.ShowError(errors.New("ещё нет готовых слайдов"), u.win)
		return
	}
	if _, err := expandExportCmd(tmpl, stems[0]); err != nil {
		dialog.ShowError(err, u.win)
		return
	}
	u.savePrefs()
	u.exportBtn.Disable()
	go func() {
		err := runExportLoop(dir, tmpl, stems, func(stem string) {
			fyne.Do(func() { u.setStatus("экспорт: " + stem) })
		})
		fyne.Do(func() {
			u.exportBtn.Enable()
			if err != nil {
				dialog.ShowError(err, u.win)
				u.setStatus("экспорт не удался")
				return
			}
			u.setStatus("экспорт завершён")
		})
	}()
}

func expandExportCmd(tmpl, stem string) (string, error) {
	cmd := strings.TrimSpace(tmpl)
	cmd = strings.TrimPrefix(cmd, "$")
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "", errors.New("пустая команда экспорта")
	}
	return strings.ReplaceAll(cmd, "%s", stem), nil
}

func runExportLoop(dir, tmpl string, stems []string, onStem func(string)) error {
	for _, stem := range stems {
		cmd, err := expandExportCmd(tmpl, stem)
		if err != nil {
			return err
		}
		if onStem != nil {
			onStem(stem)
		}
		if err := runExportCmd(dir, cmd); err != nil {
			return fmt.Errorf("%s: %w", stem, err)
		}
	}
	return nil
}

func runExportCmd(dir, command string) error {
	cmd := exec.CommandContext(context.Background(), "sh", "-c", command) //nolint:gosec // user-authored export command
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			return err
		}
		return errors.New(msg)
	}
	return nil
}
