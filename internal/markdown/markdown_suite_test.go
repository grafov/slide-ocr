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
	})

	It("rejects an empty directory", func() {
		Expect(markdown.SaveToDir(nil, markdown.ModeContinuous, "  ")).NotTo(Succeed())
	})
})
