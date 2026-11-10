VERSION := $(shell /usr/bin/plutil -extract version raw mygo.json)
MYGO := go run github.com/egoist/mygo/cmd/mygo
# Empty uses the fixed identity in mygo.json; SIGN=- opts into ad-hoc builds.
SIGN ?=
SIGN_FLAG := $(if $(SIGN),-sign "$(SIGN)",)
PREFIX := VoltPuff-$(VERSION)-macOS-universal

.PHONY: help run icons fmt test check build release

help:
	@printf 'make run      Run the menu bar app\nmake test     Run the tests\nmake check    Check formatting, tests and go vet\nmake build    Build a universal macOS app and DMG\nmake release  Create a universal DMG and SHA256SUMS in dist/\n'

run:
	go run .

icons:
	go run ./cmd/icon

fmt:
	gofmt -w main.go internal cmd

test:
	go test ./...

check: test
	@test -z "$$(gofmt -l main.go internal cmd)" || { printf 'Run make fmt\n'; exit 1; }
	go vet ./...

build:
	$(MYGO) build -platform darwin/universal $(SIGN_FLAG)

release: check build
	mkdir -p dist
	rm -f dist/VoltPuff-*-macOS-universal.dmg dist/VoltPuff-*-macOS-universal.zip dist/SHA256SUMS
	cp 'build/darwin-universal/VoltPuff $(VERSION).dmg' 'dist/$(PREFIX).dmg'
	cd dist && shasum -a 256 '$(PREFIX).dmg' > SHA256SUMS
