// Package llm talks to an OpenAI-compatible vision chat API.
package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"image"

	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"

	"github.com/grafov/slide-ocr/internal/imageutil"
)

const maxToolRounds = 4

// Config is the backend connection and sampling options.
type Config struct {
	BaseURL         string
	APIKey          string
	Model           string
	Reasoning       bool
	ReasoningEffort string
	Temperature     float32
	MaxTokens       int
}

// Client is a thin wrapper over go-openai for local and cloud endpoints.
type Client struct {
	api *openai.Client
	cfg Config
}

// New builds a client for the given OpenAI-compatible base URL.
func New(cfg Config) *Client {
	key := strings.TrimSpace(cfg.APIKey)
	if key == "" {
		key = "lm-studio"
	}
	oc := openai.DefaultConfig(key)
	oc.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	return &Client{api: openai.NewClientWithConfig(oc), cfg: cfg}
}

// ListModels returns model ids from GET /models.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	list, err := c.api.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list.Models))
	for _, m := range list.Models {
		if m.ID != "" {
			out = append(out, m.ID)
		}
	}
	return out, nil
}

// Recognize sends one slide image and optional illustration tools, returning Markdown.
func (c *Client) Recognize(ctx context.Context, img image.Image, prompt string, illustrations bool, store *imageutil.AssetStore) (string, error) {
	mime, data, err := imageutil.PrepareForLLM(img)
	if err != nil {
		return "", err
	}
	dataURI := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)

	user := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser,
		MultiContent: []openai.ChatMessagePart{
			{Type: openai.ChatMessagePartTypeText, Text: prompt},
			{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL:    dataURI,
					Detail: openai.ImageURLDetailAuto,
				},
			},
		},
	}
	msgs := []openai.ChatCompletionMessage{user}

	var tools []openai.Tool
	if illustrations {
		tools = illustrationTools()
	}

	var last string
	for round := 0; round < maxToolRounds; round++ {
		resp, err := c.api.CreateChatCompletion(ctx, c.request(msgs, tools))
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("empty chat completion")
		}
		msg := resp.Choices[0].Message
		msg.ReasoningContent = ""
		if len(msg.ToolCalls) == 0 {
			last = strings.TrimSpace(msg.Content)
			last = stripThink(last)
			if illustrations && store != nil {
				last = applyJSONCrops(last, store)
			}
			return last, nil
		}
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			Content:   msg.Content,
			ToolCalls: msg.ToolCalls,
		})
		for _, call := range msg.ToolCalls {
			result := runTool(call, store)
			msgs = append(msgs, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: call.ID,
			})
		}
	}
	return "", fmt.Errorf("too many tool rounds")
}

func (c *Client) request(msgs []openai.ChatCompletionMessage, tools []openai.Tool) openai.ChatCompletionRequest {
	req := openai.ChatCompletionRequest{
		Model:               c.cfg.Model,
		Messages:            msgs,
		Temperature:         c.cfg.Temperature,
		MaxCompletionTokens: c.cfg.MaxTokens,
		Tools:               tools,
		ChatTemplateKwargs: map[string]any{
			"enable_thinking": c.cfg.Reasoning,
		},
	}
	if c.cfg.MaxTokens == 0 {
		req.MaxCompletionTokens = 4096
	}
	if c.cfg.Reasoning {
		effort := c.cfg.ReasoningEffort
		if effort == "" {
			effort = "medium"
		}
		req.ReasoningEffort = effort
	}
	return req
}

func illustrationTools() []openai.Tool {
	cropParams := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"x":       {Type: jsonschema.Number, Description: "Левый край, доля 0..1"},
			"y":       {Type: jsonschema.Number, Description: "Верхний край, доля 0..1"},
			"w":       {Type: jsonschema.Number, Description: "Ширина, доля 0..1"},
			"h":       {Type: jsonschema.Number, Description: "Высота, доля 0..1"},
			"caption": {Type: jsonschema.String, Description: "Подпись иллюстрации"},
		},
		Required: []string{"x", "y", "w", "h"},
	}
	saveParams := jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"caption": {Type: jsonschema.String, Description: "Подпись иллюстрации"},
		},
	}
	return []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "crop_region",
				Description: "Вырезать фрагмент текущего слайда и сохранить как файл иллюстрации",
				Parameters:  cropParams,
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "save_slide_image",
				Description: "Сохранить весь текущий слайд как иллюстрацию",
				Parameters:  saveParams,
			},
		},
	}
}

type cropArgs struct {
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`
	Caption string  `json:"caption"`
}

type saveArgs struct {
	Caption string `json:"caption"`
}

func runTool(call openai.ToolCall, store *imageutil.AssetStore) string {
	if store == nil {
		return "ошибка: нет изображения для вырезания"
	}
	switch call.Function.Name {
	case "crop_region":
		var a cropArgs
		if err := json.Unmarshal([]byte(call.Function.Arguments), &a); err != nil {
			return "ошибка разбора аргументов: " + err.Error()
		}
		path, err := store.CropRegion(a.X, a.Y, a.W, a.H, a.Caption)
		if err != nil {
			return "ошибка: " + err.Error()
		}
		return "сохранено: " + path
	case "save_slide_image":
		var a saveArgs
		_ = json.Unmarshal([]byte(call.Function.Arguments), &a)
		path, err := store.SaveFull(a.Caption)
		if err != nil {
			return "ошибка: " + err.Error()
		}
		return "сохранено: " + path
	default:
		return "неизвестный инструмент: " + call.Function.Name
	}
}

var thinkRe = regexp.MustCompile(`(?s)<think>.*?</think>`)

func stripThink(s string) string {
	return strings.TrimSpace(thinkRe.ReplaceAllString(s, ""))
}

var fenceRe = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

func applyJSONCrops(text string, store *imageutil.AssetStore) string {
	raw := text
	if m := fenceRe.FindStringSubmatch(text); len(m) == 2 {
		raw = m[1]
	}
	var payload struct {
		Crops []cropArgs `json:"crops"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil || len(payload.Crops) == 0 {
		return text
	}
	var b strings.Builder
	cleaned := fenceRe.ReplaceAllString(text, "")
	b.WriteString(strings.TrimSpace(cleaned))
	for _, c := range payload.Crops {
		path, err := store.CropRegion(c.X, c.Y, c.W, c.H, c.Caption)
		if err != nil {
			continue
		}
		label := c.Caption
		if label == "" {
			label = "иллюстрация"
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "![%s](%s)", label, path)
	}
	return b.String()
}
