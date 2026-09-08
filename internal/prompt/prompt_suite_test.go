package prompt_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/grafov/slide-ocr/internal/prompt"
)

func TestPrompt(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Prompt Suite")
}

var _ = Describe("Build", func() {
	It("asks for markdown and languages", func() {
		text := prompt.Build(prompt.Options{Russian: true, English: true})
		Expect(text).To(ContainSubstring("Markdown"))
		Expect(text).To(ContainSubstring("русский"))
		Expect(text).To(ContainSubstring("английский"))
	})

	It("mentions tables and illustration tools when enabled", func() {
		text := prompt.Build(prompt.Options{
			Tables:            true,
			Illustrations:     true,
			Styles:            true,
			CompleteFragments: true,
		})
		Expect(text).To(ContainSubstring("таблиц"))
		Expect(text).To(ContainSubstring("crop_region"))
		Expect(text).To(ContainSubstring("**жирный**"))
		Expect(text).To(ContainSubstring("обрывок"))
	})

	It("includes extra languages", func() {
		text := prompt.Build(prompt.Options{ExtraLanguages: "немецкий, французский"})
		Expect(text).To(ContainSubstring("немецкий"))
		Expect(text).To(ContainSubstring("французский"))
	})
})
