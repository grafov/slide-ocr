package ui

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/grafov/slide-ocr/internal/markdown"
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
