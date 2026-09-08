package llm_test

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/grafov/slide-ocr/internal/imageutil"
	"github.com/grafov/slide-ocr/internal/llm"
)

func TestLLM(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LLM Suite")
}

var _ = Describe("Client", func() {
	It("lists models and returns markdown without reasoning", func() {
		var calls int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/models"):
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": []map[string]any{{"id": "qwen-vl"}},
				})
			case strings.HasSuffix(r.URL.Path, "/chat/completions"):
				calls++
				_ = json.NewEncoder(w).Encode(map[string]any{
					"choices": []map[string]any{
						{"message": map[string]any{
							"role":              "assistant",
							"content":           "# Слайд\n\nтекст",
							"reasoning_content": "не должно попасть в лекцию",
						}},
					},
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		DeferCleanup(srv.Close)
		c := llm.New(llm.Config{BaseURL: srv.URL + "/v1", Model: "qwen-vl", MaxTokens: 128})
		names, err := c.ListModels(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(names).To(ContainElement("qwen-vl"))
		img := tinyImage()
		text, err := c.Recognize(context.Background(), img, "распознай", false, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(text).To(ContainSubstring("# Слайд"))
		Expect(text).NotTo(ContainSubstring("не должно попасть"))
		Expect(calls).To(Equal(1))
	})

	It("runs crop_region tool then returns markdown", func() {
		var calls int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			calls++
			body, _ := io.ReadAll(r.Body)
			if calls == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"choices": []map[string]any{
						{"message": map[string]any{
							"role": "assistant",
							"tool_calls": []map[string]any{
								{
									"id":   "call_1",
									"type": "function",
									"function": map[string]any{
										"name":      "crop_region",
										"arguments": `{"x":0.1,"y":0.1,"w":0.4,"h":0.4,"caption":"схема"}`,
									},
								},
							},
						}},
					},
				})
				return
			}
			Expect(string(body)).To(ContainSubstring("call_1"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]any{
						"role":    "assistant",
						"content": "![схема](assets/slide-001-fig-1.png)",
					}},
				},
			})
		}))
		DeferCleanup(srv.Close)
		c := llm.New(llm.Config{BaseURL: srv.URL + "/v1", Model: "qwen-vl", MaxTokens: 128})
		img := tinyImage()
		store := &imageutil.AssetStore{Prefix: "slide-001", Source: img}
		text, err := c.Recognize(context.Background(), img, "с картинкой", true, store)
		Expect(err).NotTo(HaveOccurred())
		Expect(text).To(ContainSubstring("slide-001-fig-1.png"))
		Expect(store.Items).To(HaveLen(1))
		Expect(calls).To(Equal(2))
	})
})

func tinyImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 20, G: 80, B: 200, A: 255})
		}
	}
	return img
}
