package pipeline_test

import (
	"context"
	"image"
	"image/color"
	"path/filepath"
	"sync/atomic"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kovidgoyal/imaging"

	"github.com/grafov/slide-ocr/internal/imageutil"
	"github.com/grafov/slide-ocr/internal/markdown"
	"github.com/grafov/slide-ocr/internal/pipeline"
	"github.com/grafov/slide-ocr/internal/session"
)

func TestPipeline(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pipeline Suite")
}

type stubRec struct {
	n atomic.Int32
}

func (s *stubRec) Recognize(_ context.Context, _ image.Image, _ string, _ bool, _ *imageutil.AssetStore) (string, error) {
	i := s.n.Add(1)
	if i == 1 {
		return "Общий заголовок. Первый пункт.", nil
	}
	return "Совсем другая тема слайда про сети.", nil
}

var _ = Describe("Run", func() {
	It("skips a visual duplicate and OCR's the rest", func() {
		dir := GinkgoT().TempDir()
		a := patterned(0)
		b := patterned(0)
		c := patterned(1)
		p1 := filepath.Join(dir, "a.png")
		p2 := filepath.Join(dir, "b.png")
		p3 := filepath.Join(dir, "c.png")
		Expect(imaging.Save(a, p1)).To(Succeed())
		Expect(imaging.Save(b, p2)).To(Succeed())
		Expect(imaging.Save(c, p3)).To(Succeed())
		rec := &stubRec{}
		res, err := pipeline.Run(context.Background(), pipeline.Request{
			Slides: []session.Slide{
				{ID: "1", Path: p1, Name: "a.png"},
				{ID: "2", Path: p2, Name: "b.png"},
				{ID: "3", Path: p3, Name: "c.png"},
			},
			Prompt:       "x",
			PDQThreshold: 0,
			Recognizer:   rec,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(int(rec.n.Load())).To(Equal(2))
		Expect(res.Slides).To(HaveLen(3))
		Expect(res.Slides[1].Skip).To(BeTrue())
		Expect(res.Slides[0].ID).To(Equal("1"))
		Expect(res.Slides[2].ID).To(Equal("3"))
		Expect(res.Slides[0].Text).To(ContainSubstring("заголовок"))
		Expect(res.Slides[2].Text).To(ContainSubstring("сети"))
	})

	It("emits OnSlide after each OCR before the next image", func() {
		dir := GinkgoT().TempDir()
		a := patterned(0)
		c := patterned(1)
		p1 := filepath.Join(dir, "a.png")
		p3 := filepath.Join(dir, "c.png")
		Expect(imaging.Save(a, p1)).To(Succeed())
		Expect(imaging.Save(c, p3)).To(Succeed())
		var slidesSeen atomic.Int32
		rec := &gateRec{seen: &slidesSeen}
		_, err := pipeline.Run(context.Background(), pipeline.Request{
			Slides: []session.Slide{
				{ID: "1", Path: p1, Name: "a.png"},
				{ID: "3", Path: p3, Name: "c.png"},
			},
			Prompt:       "x",
			PDQThreshold: 0,
			Recognizer:   rec,
			OnSlide: func(s markdown.Slide) {
				if !s.Skip && s.Text != "" {
					slidesSeen.Add(1)
				}
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(int(rec.n.Load())).To(Equal(2))
		Expect(int(slidesSeen.Load())).To(Equal(2))
		Expect(rec.seenAtSecond.Load()).To(BeNumerically(">=", 1))
	})
})

type gateRec struct {
	n            atomic.Int32
	seen         *atomic.Int32
	seenAtSecond atomic.Int32
}

func (s *gateRec) Recognize(_ context.Context, _ image.Image, _ string, _ bool, _ *imageutil.AssetStore) (string, error) {
	i := s.n.Add(1)
	if i == 2 {
		s.seenAtSecond.Store(s.seen.Load())
	}
	if i == 1 {
		return "Общий заголовок. Первый пункт.", nil
	}
	return "Совсем другая тема слайда про сети.", nil
}

func patterned(kind int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			on := false
			if kind == 0 {
				on = y < 128
			} else {
				on = x < 128
			}
			v := uint8(0)
			if on {
				v = 255
			}
			img.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: 255})
		}
	}
	return img
}
