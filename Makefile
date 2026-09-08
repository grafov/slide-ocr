##
# slide-ocr — desktop GUI for lecture-slide OCR
#
# @file
# @version 0.1

PRJ=slide-ocr
APP=slide-ocr
BINDIR=build

PREFIX?=/usr/local/bin

SUDO?=

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-dev")
COMMIT := $(shell git describe --always --dirty 2>/dev/null || echo "unknown")
FLAGS := -buildvcs=false -ldflags "-X main.version=$(VERSION) -X main.gitCommit=$(COMMIT)"

.PHONY: all
all: build

.PHONY: build
build:
	mkdir -p $(BINDIR)
	$(foreach dir,$(wildcard cmd/*), echo "$(dir) building..."; go build $(FLAGS) -o $(BINDIR)/ ./$(dir);)

.PHONY: test
test:
	go tool ginkgo ./...

.PHONY: run
run: build
	./$(BINDIR)/$(APP)

.PHONY: run-log
run-log: tidy build
	SLOG_LEVEL=debug ./$(BINDIR)/$(APP)

.PHONY: run-race
run-race: tidy
	go run -race $(FLAGS) ./cmd/$(APP)

.PHONY: lint
lint:
	go tool golangci-lint run ./...

.PHONY: lint-fix
lint-fix:
	go tool golangci-lint run --fix ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: sloc
sloc:
	cloc * >sloc.stats

.PHONY: clean
clean:
	go clean
	rm -rf $(BINDIR)

# end
