# Build release artifacts into dist/.
#
#   make cli       command-line tool for macOS, Linux and Windows (pure Go, builds anywhere)
#   make macos     GUI as a universal macOS .app (run on macOS)
#   make windows   GUI for Windows amd64 (on Windows with MSYS2: make windows MINGW_CC=gcc,
#                  or cross-compile with a MinGW-w64 gcc, e.g. `brew install mingw-w64`)
#   make linux     GUI for Linux on the current architecture (run on Linux)
#
# Fyne uses cgo, so each GUI target needs a C compiler for that platform.

APP      := DeS Save Editor
SLUG     := DeS-Save-Editor
VERSION  := $(shell sed -n 's/^ *Version *= *"\(.*\)"/\1/p' FyneApp.toml)
DIST     := dist
TMP      := $(DIST)/.tmp
LDFLAGS  := -s -w
FYNE     ?= $(shell go env GOPATH)/bin/fyne
MINGW_CC ?= x86_64-w64-mingw32-gcc

.PHONY: all test cli macos windows linux clean

all: cli macos

test:
	go test ./...

cli:
	@mkdir -p $(DIST)
	@for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do \
		os=$${target%/*}; arch=$${target#*/}; ext=""; [ $$os = windows ] && ext=.exe; \
		echo "dessave $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" \
			-o $(DIST)/dessave-$(VERSION)-$$os-$$arch$$ext ./cmd/dessave || exit 1; \
	done

macos:
	@mkdir -p $(TMP)
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(TMP)/editor-arm64 .
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(TMP)/editor-amd64 .
	lipo -create -output $(TMP)/des-save-editor $(TMP)/editor-arm64 $(TMP)/editor-amd64
	rm -rf "$(DIST)/$(APP).app"
	$(FYNE) package --os darwin --release --exe $(TMP)/des-save-editor --name "$(APP)"
	mv "$(APP).app" "$(DIST)/"
	cd $(DIST) && rm -f $(SLUG)-$(VERSION)-macos-universal.zip && \
		ditto -c -k --keepParent "$(APP).app" $(SLUG)-$(VERSION)-macos-universal.zip
	rm -rf $(TMP)

windows:
	@mkdir -p $(DIST)
	CC=$(MINGW_CC) CGO_ENABLED=1 GOOS=windows GOARCH=amd64 $(FYNE) package --os windows --release --name "$(APP)"
	mv "$(APP).exe" $(DIST)/$(SLUG)-$(VERSION)-windows-amd64.exe

linux:
	@mkdir -p $(DIST)
	$(FYNE) package --os linux --release --name "$(APP)"
	mv "$(APP).tar.xz" $(DIST)/$(SLUG)-$(VERSION)-linux-$(shell go env GOARCH).tar.xz

clean:
	rm -rf $(DIST)
