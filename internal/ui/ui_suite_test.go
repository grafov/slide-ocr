package ui

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/grafov/slide-ocr/internal/markdown"
	"github.com/grafov/slide-ocr/internal/session"
)

func TestUI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "UI Suite")
}

var _ = Describe("shouldOpenDialog", func() {
	It("opens only when the press stayed on the same row", func() {
		Expect(shouldOpenDialog(2, 2)).To(BeTrue())
		Expect(shouldOpenDialog(2, 5)).To(BeFalse())
		Expect(shouldOpenDialog(-1, -1)).To(BeFalse())
	})
})

var _ = Describe("hasUnsaved", func() {
	It("detects fragments that were not saved", func() {
		slides := []markdown.Slide{
			{ID: "1", Text: "one"},
			{ID: "2", Text: "two"},
		}
		Expect(hasUnsaved(slides, nil)).To(BeTrue())
		Expect(hasUnsaved(slides, map[string]bool{"1": true, "2": true})).To(BeFalse())
		Expect(hasUnsaved(slides, map[string]bool{"1": true})).To(BeTrue())
	})
})

var _ = Describe("orderByDoc", func() {
	It("follows snapshot ids", func() {
		result := []markdown.Slide{
			{ID: "b", Text: "two"},
			{ID: "a", Text: "one"},
		}
		got := markdown.OrderByIDs([]string{"a", "b"}, result)
		Expect(got[0].ID).To(Equal("a"))
		Expect(got[1].ID).To(Equal("b"))
	})
})

var _ = Describe("formatOCRDuration", func() {
	It("uses seconds below a minute and a clock after", func() {
		Expect(formatOCRDuration(12300 * time.Millisecond)).To(Equal("12.3 с"))
		Expect(formatOCRDuration(65 * time.Second)).To(Equal("1:05"))
	})
})

var _ = Describe("formatRunLine", func() {
	It("shows elapsed time and an ETA after the first completed slide", func() {
		Expect(formatRunLine("распознавание a.png", 5*time.Second, 0, 4, false, false)).To(Equal("распознавание a.png · прошло 0:05"))
		Expect(formatRunLine("распознавание b.png", 20*time.Second, 2, 6, false, false)).To(Equal("распознавание b.png · прошло 0:20 · осталось ~0:40"))
		Expect(formatRunLine("", 90*time.Second, 0, 0, true, false)).To(Equal("пауза · прошло 1:30"))
		Expect(formatRunLine("готово, слайдов в выводе: 3", 222*time.Second, 0, 0, false, true)).To(Equal("готово, слайдов в выводе: 3 · 3:42"))
	})
})

var _ = Describe("formatSlideMeta", func() {
	It("joins size with an em dash when OCR has not run", func() {
		Expect(formatSlideMeta(session.Slide{Size: 2048})).To(Equal("2.0 KB · —"))
	})
})
