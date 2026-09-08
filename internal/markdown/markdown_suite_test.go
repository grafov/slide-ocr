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
