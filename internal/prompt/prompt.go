// Package prompt builds the Russian system prompt from recognition options.
package prompt

import (
	"strings"
)

// Options control which instructions are added to the OCR prompt.
type Options struct {
	Russian           bool
	English           bool
	ExtraLanguages    string
	Illustrations     bool
	Tables            bool
	Styles            bool
	CompleteFragments bool
}

// Build returns a Russian prompt describing how to turn a slide photo into Markdown.
func Build(opts Options) string {
	var b strings.Builder
	b.WriteString("Ты распознаёшь текст со фотографии слайда лекции.\n")
	b.WriteString("Верни только Markdown итоговой расшифровки этого слайда, без предисловий и без служебных комментариев.\n")
	b.WriteString("Не включай цепочку рассуждений, теги think и содержимое reasoning — только готовый текст лекции.\n")
	b.WriteString("Сохраняй структуру: заголовки, списки, нумерацию, абзацы.\n")
	b.WriteString("Игнорируй элементы оформления шаблона, если это не содержание (номера слайдов, логотипы в углу без смысла).\n")

	langs := languages(opts)
	if len(langs) > 0 {
		b.WriteString("Языки распознавания: ")
		b.WriteString(strings.Join(langs, ", "))
		b.WriteString(". Сохраняй язык оригинала, не переводи.\n")
	}

	if opts.Tables {
		b.WriteString("Если на слайде есть таблица, передай её Markdown-таблицей (| колонка | ... |), максимально точно по ячейкам.\n")
	} else {
		b.WriteString("Таблицы не восстанавливай как сетку: при необходимости кратко перескажи содержимое обычным текстом.\n")
	}

	if opts.Styles {
		b.WriteString("Выделяй явно жирный текст как **жирный** и курсив как *курсив*, если это видно на слайде.\n")
	}

	if opts.CompleteFragments {
		b.WriteString("Если слово обрезано краем кадра, восстанови очевидный обрывок по видимому контексту этой же строки.\n")
		b.WriteString("Не выдумывай факты, формулы и абзацы, которых нет на слайде. Если восстановить нельзя — оставь как есть или пометь […].\n")
	} else {
		b.WriteString("Не дописывай обрезанные слова: копируй только то, что видно.\n")
	}

	if opts.Illustrations {
		b.WriteString("Если на слайде есть иллюстрация, схема или график, вызови инструмент crop_region (доли 0..1 от ширины и высоты кадра) или save_slide_image для целого кадра.\n")
		b.WriteString("После сохранения вставь в Markdown ссылку вида ![подпись](путь_из_ответа_инструмента).\n")
		b.WriteString("Не описывай картинку длинным текстом, если её удалось вырезать. Если инструмент недоступен, кратко опиши фигуру в одном предложении.\n")
	} else {
		b.WriteString("Иллюстрации не вырезай и не описывай подробно, только текст.\n")
	}

	return strings.TrimSpace(b.String()) + "\n"
}

func languages(opts Options) []string {
	var out []string
	if opts.Russian {
		out = append(out, "русский")
	}
	if opts.English {
		out = append(out, "английский")
	}
	extra := strings.TrimSpace(opts.ExtraLanguages)
	if extra != "" {
		for _, part := range strings.Split(extra, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}
