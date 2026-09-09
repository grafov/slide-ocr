package session_test

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kovidgoyal/imaging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/grafov/slide-ocr/internal/session"
)

func TestSession(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Session Suite")
}

var _ = Describe("Document", func() {
	It("records file size and finds a slide by id", func() {
		dir := GinkgoT().TempDir()
		p := filepath.Join(dir, "slide.png")
		img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		img.SetNRGBA(0, 0, color.NRGBA{A: 255})
		Expect(imaging.Save(img, p)).To(Succeed())
		info, err := os.Stat(p)
		Expect(err).NotTo(HaveOccurred())

		doc := &session.Document{}
		Expect(doc.AddPaths([]string{p})).To(Equal(1))
		s, ok := doc.At(0)
		Expect(ok).To(BeTrue())
		Expect(s.Included).To(BeTrue())
		Expect(s.Size).To(Equal(info.Size()))
		Expect(s.Size).To(BeNumerically(">", 0))
		idx, ok := doc.IndexByID(s.ID)
		Expect(ok).To(BeTrue())
		Expect(idx).To(Equal(0))
		_, ok = doc.IndexByID("missing")
		Expect(ok).To(BeFalse())
	})

	It("clears OCR state without dropping the include flag", func() {
		dir := GinkgoT().TempDir()
		p := filepath.Join(dir, "slide.png")
		img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		img.SetNRGBA(0, 0, color.NRGBA{A: 255})
		Expect(imaging.Save(img, p)).To(Succeed())
		doc := &session.Document{}
		Expect(doc.AddPaths([]string{p})).To(Equal(1))
		s, _ := doc.At(0)
		doc.SetText(s.ID, "hello")
		doc.SetIncluded(s.ID, false)
		s, _ = doc.At(0)
		Expect(s.Included).To(BeFalse())
		Expect(s.Text).To(BeEmpty())
		Expect(s.Status).To(Equal(session.StatusPending))
		doc.SetIncluded(s.ID, true)
		doc.SetText(s.ID, "hello")
		doc.ClearOutput()
		s, _ = doc.At(0)
		Expect(s.Included).To(BeTrue())
		Expect(s.Text).To(BeEmpty())
		Expect(s.Status).To(Equal(session.StatusPending))
	})

	It("marks no-text and keeps the include flag", func() {
		dir := GinkgoT().TempDir()
		p := filepath.Join(dir, "slide.png")
		img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		img.SetNRGBA(0, 0, color.NRGBA{A: 255})
		Expect(imaging.Save(img, p)).To(Succeed())
		doc := &session.Document{}
		Expect(doc.AddPaths([]string{p})).To(Equal(1))
		s, _ := doc.At(0)
		doc.MarkNoText(s.ID, "![x](assets/x.png)")
		s, _ = doc.At(0)
		Expect(s.Included).To(BeTrue())
		Expect(s.Status).To(Equal(session.StatusNoText))
		doc.ClearOutput()
		s, _ = doc.At(0)
		Expect(s.Included).To(BeTrue())
		Expect(s.Status).To(Equal(session.StatusPending))
		Expect(s.OCRDuration).To(Equal(time.Duration(0)))
	})

	It("stores OCR duration for a slide", func() {
		dir := GinkgoT().TempDir()
		p := filepath.Join(dir, "slide.png")
		img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		img.SetNRGBA(0, 0, color.NRGBA{A: 255})
		Expect(imaging.Save(img, p)).To(Succeed())
		doc := &session.Document{}
		Expect(doc.AddPaths([]string{p})).To(Equal(1))
		s, _ := doc.At(0)
		doc.SetOCRDuration(s.ID, 2500*time.Millisecond)
		s, _ = doc.At(0)
		Expect(s.OCRDuration).To(Equal(2500 * time.Millisecond))
	})
})
