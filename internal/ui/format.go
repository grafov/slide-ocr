package ui

import (
	"fmt"
	"time"

	"github.com/grafov/slide-ocr/internal/session"
)

func formatOCRDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d >= time.Minute {
		return formatClock(d)
	}
	return fmt.Sprintf("%.1f с", d.Seconds())
}

func formatClock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Round(time.Second) / time.Second)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func slideTimeText(s session.Slide) string {
	if s.Status == session.StatusRunning && !s.OCRStarted.IsZero() {
		return formatOCRDuration(time.Since(s.OCRStarted))
	}
	if s.OCRDuration > 0 {
		return formatOCRDuration(s.OCRDuration)
	}
	return "—"
}

func formatSlideMeta(s session.Slide) string {
	return formatFileSize(s.Size) + " · " + slideTimeText(s)
}

func formatRunLine(msg string, elapsed time.Duration, current, total int, paused, done bool) string {
	clock := formatClock(elapsed)
	if done {
		if msg == "" {
			return clock
		}
		return msg + " · " + clock
	}
	if paused {
		return "пауза · прошло " + clock
	}
	line := msg
	if line != "" {
		line += " · "
	}
	line += "прошло " + clock
	if current > 0 && total > current {
		remain := elapsed * time.Duration(total-current) / time.Duration(current)
		line += " · осталось ~" + formatClock(remain)
	}
	return line
}
