# Changelog

Все заметные изменения проекта документируются в этом файле.

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/).

## [Unreleased]

### Added

- Desktop GUI на Fyne с тремя вкладками: вход (пакет картинок, сортировка и перестановка), распознавание через OpenAI-compatible VLM, вывод Markdown.
- Локальный tool `crop_region` / `save_slide_image` для вырезания иллюстраций в sidecar-файлы.
- Тихий дедуп одинаковых кадров (PDQ) и склейка overlapping-текста соседних слайдов.
