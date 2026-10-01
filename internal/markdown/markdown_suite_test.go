package markdown_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/grafov/slide-ocr/internal/imageutil"
	"github.com/grafov/slide-ocr/internal/markdown"
)

func TestMarkdown(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Markdown Suite")
}

var _ = Describe("Stitch", func() {
	slides := []markdown.Slide{
		{Name: "a.png", Text: "# One"},
		{Name: "b.png", Text: "# Two"},
		{Name: "c.png", Text: "skip me", Skip: true},
	}

	It("joins with blank lines by default", func() {
		Expect(markdown.Stitch(slides, markdown.ModeContinuous)).To(Equal("# One\n\n# Two\n"))
	})

	It("inserts a separator", func() {
		Expect(markdown.Stitch(slides, markdown.ModeSeparator)).To(ContainSubstring("-----\n"))
	})

	It("keeps a NoText slot as an empty string", func() {
		got := markdown.Stitch([]markdown.Slide{
			{Name: "a.png", Text: "# One"},
			{Name: "b.png", NoText: true, Text: ""},
			{Name: "c.png", Text: "# Two"},
		}, markdown.ModeContinuous)
		Expect(got).To(Equal("# One\n\n\n\n# Two\n"))
	})
})

var _ = Describe("OrderByIDs", func() {
	It("orders slides by the given ids", func() {
		slides := []markdown.Slide{
			{ID: "2", Text: "b"},
			{ID: "1", Text: "a"},
		}
		got := markdown.OrderByIDs([]string{"1", "2"}, slides)
		Expect(got).To(HaveLen(2))
		Expect(got[0].ID).To(Equal("1"))
		Expect(got[1].ID).To(Equal("2"))
	})
})

var _ = Describe("HasSlideText", func() {
	It("ignores image links and whitespace", func() {
		Expect(markdown.HasSlideText("")).To(BeFalse())
		Expect(markdown.HasSlideText("   ")).To(BeFalse())
		Expect(markdown.HasSlideText("![Нет текста](assets/x.png)")).To(BeFalse())
		Expect(markdown.HasSlideText("# Заголовок")).To(BeTrue())
		Expect(markdown.HasSlideText("см. ![рис](assets/a.png)")).To(BeTrue())
	})
})

var _ = Describe("Save", func() {
	It("writes markdown and sidecar assets", func() {
		dir := GinkgoT().TempDir()
		path := filepath.Join(dir, "lecture.md")
		slides := []markdown.Slide{
			{
				Name: "s1.png",
				Text: "см. ![рис](assets/slide-001-fig-1.png)",
				Assets: []imageutil.Asset{
					{FileName: "slide-001-fig-1.png", RelPath: "assets/slide-001-fig-1.png", Data: []byte("png")},
				},
			},
		}
		Expect(markdown.Save(slides, markdown.ModeContinuous, path)).To(Succeed())
		body, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(ContainSubstring("lecture.assets/slide-001-fig-1.png"))
		Expect(filepath.Join(dir, "lecture.assets", "slide-001-fig-1.png")).To(BeAnExistingFile())
	})
})

var _ = Describe("MarkdownStems", func() {
	It("uses recognized-text for combined modes", func() {
		slides := []markdown.Slide{{Name: "a.png", Text: "# One"}}
		Expect(markdown.MarkdownStems(slides, markdown.ModeContinuous)).To(Equal([]string{"recognized-text"}))
		Expect(markdown.MarkdownStems(slides, markdown.ModeSeparator)).To(Equal([]string{"recognized-text"}))
		Expect(markdown.MarkdownStems(nil, markdown.ModeContinuous)).To(Equal([]string{"recognized-text"}))
	})

	It("names separate files like SaveToDir and skips omitted slides", func() {
		slides := []markdown.Slide{
			{Name: "shot.png", Text: "one"},
			{Name: "skip.png", Text: "no", Skip: true},
			{Name: "shot.jpg", Text: "two"},
			{Name: "blank.png", Text: "  "},
			{Name: "other.png", Text: "three"},
		}
		Expect(markdown.MarkdownStems(slides, markdown.ModeSeparateFiles)).To(Equal([]string{"shot", "shot-2", "other"}))
	})
})

var _ = Describe("SaveToDir", func() {
	It("writes recognized-text.md for continuous mode", func() {
		dir := GinkgoT().TempDir()
		slides := []markdown.Slide{
			{Name: "a.png", Text: "# One"},
			{Name: "b.png", Text: "# Two"},
		}
		Expect(markdown.SaveToDir(slides, markdown.ModeContinuous, dir)).To(Succeed())
		path := filepath.Join(dir, markdown.CombinedFileName)
		body, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(ContainSubstring("# One"))
		Expect(string(body)).To(ContainSubstring("# Two"))
	})

	It("names separate files after source stems and suffixes collisions", func() {
		dir := GinkgoT().TempDir()
		slides := []markdown.Slide{
			{Name: "shot.png", Text: "one"},
			{Name: "shot.jpg", Text: "two"},
			{Name: "other.png", Text: "three"},
		}
		Expect(markdown.SaveToDir(slides, markdown.ModeSeparateFiles, dir)).To(Succeed())
		Expect(filepath.Join(dir, "shot.md")).To(BeAnExistingFile())
		Expect(filepath.Join(dir, "shot-2.md")).To(BeAnExistingFile())
		Expect(filepath.Join(dir, "other.md")).To(BeAnExistingFile())
		Expect(filepath.Join(dir, "001-shot.md")).NotTo(BeAnExistingFile())
		for _, stem := range markdown.MarkdownStems(slides, markdown.ModeSeparateFiles) {
			Expect(filepath.Join(dir, stem+".md")).To(BeAnExistingFile())
		}
	})

	It("rejects an empty directory", func() {
		Expect(markdown.SaveToDir(nil, markdown.ModeContinuous, "  ")).NotTo(Succeed())
	})

	It("writes an empty markdown file for a NoText slide", func() {
		dir := GinkgoT().TempDir()
		slides := []markdown.Slide{
			{Name: "empty.png", NoText: true, Text: ""},
			{Name: "ok.png", Text: "# Two"},
		}
		Expect(markdown.SaveToDir(slides, markdown.ModeSeparateFiles, dir)).To(Succeed())
		emptyPath := filepath.Join(dir, "empty.md")
		Expect(emptyPath).To(BeAnExistingFile())
		body, err := os.ReadFile(emptyPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("\n"))
		Expect(filepath.Join(dir, "ok.md")).To(BeAnExistingFile())
	})
})
