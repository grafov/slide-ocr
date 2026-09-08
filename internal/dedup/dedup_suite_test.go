package dedup_test

import (
	"image"
	"image/color"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/grafov/slide-ocr/internal/dedup"
)

func TestDedup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dedup Suite")
}

var _ = Describe("MergeOverlapping", func() {
	It("keeps the longer incremental slide", func() {
		a := "Тема лекции. Пункт один. Пункт два. Пункт три."
		b := "Тема лекции. Пункт один. Пункт два. Пункт три. Пункт четыре и пять."
		out := dedup.MergeOverlapping([]string{a, b})
		Expect(out).To(HaveLen(1))
		Expect(out[0]).To(Equal(b))
	})

	It("keeps distinct slides", func() {
		out := dedup.MergeOverlapping([]string{
			"Введение в графы и деревья поиска.",
			"Алгоритм Дейкстры на взвешенных графах.",
		})
		Expect(out).To(HaveLen(2))
	})
})

var _ = Describe("Overlaps", func() {
	It("detects identical normalized text", func() {
		Expect(dedup.Overlaps("Hello  World", "hello world")).To(BeTrue())
	})
})

var _ = Describe("PlanVisual", func() {
	It("skips a copied frame", func() {
		a := patterned(0)
		b := patterned(0)
		c := patterned(1)
		plan := dedup.PlanVisual([]image.Image{a, b, c}, 0)
		Expect(plan).To(HaveLen(3))
		Expect(plan[0].Skip).To(BeFalse())
		Expect(plan[1].Skip).To(BeTrue())
		Expect(plan[2].Skip).To(BeFalse())
	})
})

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
