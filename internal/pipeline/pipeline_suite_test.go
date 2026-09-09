package pipeline_test

import (
	"context"
	"image"
	"image/color"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

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
				{ID: "1", Path: p1, Name: "a.png", Included: true},
				{ID: "2", Path: p2, Name: "b.png", Included: true},
				{ID: "3", Path: p3, Name: "c.png", Included: true},
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
				{ID: "1", Path: p1, Name: "a.png", Included: true},
				{ID: "3", Path: p3, Name: "c.png", Included: true},
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

	It("does not OCR a finished slide or an unchecked one", func() {
		dir := GinkgoT().TempDir()
		a := patterned(0)
		c := patterned(1)
		p1 := filepath.Join(dir, "a.png")
		p3 := filepath.Join(dir, "c.png")
		Expect(imaging.Save(a, p1)).To(Succeed())
		Expect(imaging.Save(c, p3)).To(Succeed())
		rec := &stubRec{}
		res, err := pipeline.Run(context.Background(), pipeline.Request{
			Slides: []session.Slide{
				{ID: "1", Path: p1, Name: "a.png", Included: true, Status: session.StatusDone, Text: "уже готово"},
				{ID: "3", Path: p3, Name: "c.png", Included: false},
			},
			Prompt:       "x",
			PDQThreshold: 0,
			Recognizer:   rec,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(int(rec.n.Load())).To(Equal(0))
		Expect(res.Slides[0].Text).To(Equal("уже готово"))
		Expect(res.Slides[1].Skip).To(BeTrue())
	})

	It("RecognizeOne OCRs a single included slide", func() {
		dir := GinkgoT().TempDir()
		a := patterned(0)
		p1 := filepath.Join(dir, "a.png")
		Expect(imaging.Save(a, p1)).To(Succeed())
		rec := &stubRec{}
		md, err := pipeline.RecognizeOne(context.Background(), pipeline.OneRequest{
			Slide:      session.Slide{ID: "1", Path: p1, Name: "a.png", Included: true},
			Prompt:     "x",
			Recognizer: rec,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(int(rec.n.Load())).To(Equal(1))
		Expect(md.Text).To(ContainSubstring("заголовок"))
		Expect(md.ID).To(Equal("1"))
	})

	It("returns a canceled slide to pending", func() {
		dir := GinkgoT().TempDir()
		a := patterned(0)
		p1 := filepath.Join(dir, "a.png")
		Expect(imaging.Save(a, p1)).To(Succeed())
		started := make(chan struct{})
		rec := &cancelRec{started: started}
		ctx, cancel := context.WithCancel(context.Background())
		var last session.Status
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = pipeline.Run(ctx, pipeline.Request{
				Slides: []session.Slide{
					{ID: "1", Path: p1, Name: "a.png", Included: true},
				},
				Prompt:       "x",
				PDQThreshold: 0,
				Recognizer:   rec,
				SetStatus: func(_ string, status session.Status, _ string) {
					last = status
				},
			})
		}()
		Eventually(started).Should(BeClosed())
		cancel()
		Eventually(done).Should(BeClosed())
		Expect(last).To(Equal(session.StatusPending))
	})

	It("picks the new top pending slide after reorder", func() {
		dir := GinkgoT().TempDir()
		p1 := filepath.Join(dir, "a.png")
		p2 := filepath.Join(dir, "b.png")
		p3 := filepath.Join(dir, "c.png")
		Expect(imaging.Save(patterned(0), p1)).To(Succeed())
		Expect(imaging.Save(patterned(1), p2)).To(Succeed())
		Expect(imaging.Save(patterned(2), p3)).To(Succeed())
		slides := []session.Slide{
			{ID: "a", Path: p1, Name: "a.png", Included: true},
			{ID: "b", Path: p2, Name: "b.png", Included: true},
			{ID: "c", Path: p3, Name: "c.png", Included: true},
		}
		rec := &idRec{}
		_, err := pipeline.Run(context.Background(), pipeline.Request{
			Snapshot: func() []session.Slide {
				out := make([]session.Slide, len(slides))
				copy(out, slides)
				return out
			},
			Prompt:       "x",
			PDQThreshold: 0,
			Recognizer:   rec,
			OnSlide: func(s markdown.Slide) {
				if rec.n.Load() == 1 {
					slides = []session.Slide{slides[2], slides[1], slides[0]}
				}
				_ = s
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(rec.ids).To(Equal([]string{"a", "c", "b"}))
	})

	It("skips a pending duplicate of a Done slide below it", func() {
		dir := GinkgoT().TempDir()
		p1 := filepath.Join(dir, "a.png")
		p2 := filepath.Join(dir, "b.png")
		Expect(imaging.Save(patterned(0), p1)).To(Succeed())
		Expect(imaging.Save(patterned(0), p2)).To(Succeed())
		rec := &stubRec{}
		res, err := pipeline.Run(context.Background(), pipeline.Request{
			Slides: []session.Slide{
				{ID: "2", Path: p2, Name: "b.png", Included: true},
				{ID: "1", Path: p1, Name: "a.png", Included: true, Status: session.StatusDone, Text: "уже готово"},
			},
			Prompt:       "x",
			PDQThreshold: 0,
			Recognizer:   rec,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(int(rec.n.Load())).To(Equal(0))
		Expect(res.Slides[0].Skip).To(BeTrue())
		Expect(res.Slides[1].Text).To(Equal("уже готово"))
	})

	It("marks empty OCR as NoText and can embed the full slide", func() {
		dir := GinkgoT().TempDir()
		p1 := filepath.Join(dir, "a.png")
		Expect(imaging.Save(patterned(0), p1)).To(Succeed())
		doc := &session.Document{}
		Expect(doc.AddPaths([]string{p1})).To(Equal(1))
		s, _ := doc.At(0)
		rec := &emptyRec{}
		res, err := pipeline.Run(context.Background(), pipeline.Request{
			Snapshot:      doc.Snapshot,
			Prompt:        "x",
			EmptyAsImages: true,
			PDQThreshold:  0,
			Recognizer:    rec,
			SetStatus:     doc.SetStatus,
			OnSlide: func(md markdown.Slide) {
				if md.NoText {
					doc.MarkNoText(md.ID, md.Text)
				}
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Slides[0].NoText).To(BeTrue())
		Expect(res.Slides[0].Text).To(ContainSubstring("![Нет текста]"))
		Expect(res.Slides[0].Assets).NotTo(BeEmpty())
		got, ok := doc.At(0)
		Expect(ok).To(BeTrue())
		Expect(got.ID).To(Equal(s.ID))
		Expect(got.Included).To(BeTrue())
		Expect(got.Status).To(Equal(session.StatusNoText))
	})

	It("records OCR duration after a VLM call", func() {
		dir := GinkgoT().TempDir()
		p1 := filepath.Join(dir, "a.png")
		Expect(imaging.Save(patterned(0), p1)).To(Succeed())
		rec := &delayRec{delay: 20 * time.Millisecond}
		var got time.Duration
		res, err := pipeline.Run(context.Background(), pipeline.Request{
			Slides: []session.Slide{
				{ID: "1", Path: p1, Name: "a.png", Included: true},
			},
			Prompt:       "x",
			PDQThreshold: 0,
			Recognizer:   rec,
			SetDuration: func(_ string, d time.Duration) {
				got = d
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Slides[0].Text).To(ContainSubstring("заголовок"))
		Expect(got).To(BeNumerically(">=", 20*time.Millisecond))
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

type cancelRec struct {
	started chan struct{}
}

func (c *cancelRec) Recognize(ctx context.Context, _ image.Image, _ string, _ bool, _ *imageutil.AssetStore) (string, error) {
	close(c.started)
	<-ctx.Done()
	return "", ctx.Err()
}

type idRec struct {
	n   atomic.Int32
	ids []string
}

func (r *idRec) Recognize(_ context.Context, img image.Image, _ string, _ bool, _ *imageutil.AssetStore) (string, error) {
	r.n.Add(1)
	id := patternID(img)
	r.ids = append(r.ids, id)
	switch id {
	case "a":
		return "Текст слайда альфа достаточно длинный чтобы не схлопнуться.", nil
	case "c":
		return "Тема про сети и маршрутизацию пакетов в другой лекции.", nil
	default:
		return "Совсем отдельный сюжет про базы данных и транзакции.", nil
	}
}

func patternID(img image.Image) string {
	r1, _, _, _ := img.At(200, 50).RGBA()
	r2, _, _, _ := img.At(50, 200).RGBA()
	if r1 > 0x8000 && r2 < 0x8000 {
		return "a"
	}
	if r2 > 0x8000 && r1 < 0x8000 {
		return "b"
	}
	return "c"
}

type emptyRec struct{}

func (emptyRec) Recognize(_ context.Context, _ image.Image, _ string, _ bool, _ *imageutil.AssetStore) (string, error) {
	return "   ", nil
}

type delayRec struct {
	delay time.Duration
	stub  stubRec
}

func (d *delayRec) Recognize(ctx context.Context, img image.Image, prompt string, illust bool, store *imageutil.AssetStore) (string, error) {
	time.Sleep(d.delay)
	return d.stub.Recognize(ctx, img, prompt, illust, store)
}

func patterned(kind int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			on := false
			if kind == 0 {
				on = y < 128
			} else if kind == 1 {
				on = x < 128
			} else {
				on = x+y < 200
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
