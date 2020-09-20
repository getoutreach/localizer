# go option
GO             ?= go
SHELL          := /usr/bin/env bash
GOOS           := $(shell go env GOOS)
GOARCH         := $(shell go env GOARCH)
PKG            := $(GO) mod download
# TODO(jaredallard): infer from Git tag
APP_VERSION    := 1.0.0-$(shell git rev-parse HEAD)
LDFLAGS        := -w -s
GOFLAGS        :=
GOPROXY        := https://proxy.golang.org
GO_EXTRA_FLAGS := -v
TAGS           :=
BINDIR         := $(CURDIR)/bin
BIN_NAME       := bootstraper
PKGDIR         := github.com/jaredallard/localizer
CGO_ENABLED    := 1
BENCH_FLAGS    := "-bench=Bench $(BENCH_FLAGS)"
TEST_TAGS      ?= tm_test

.PHONY: default
default: build

.PHONY: release
release:
	./scripts/gobin.sh github.com/goreleaser/goreleaser

.PHONY: pre-commit
pre-commit: fmt

.PHONY: build
build: gogenerate gobuild

.PHONY: test
test:
	GOPROXY=$(GOPROXY) ./scripts/test.sh

.PHONY: docs
docs:
	./scripts/docs.sh

.PHONY: dep
dep:
	@echo " :: Installing dependencies using '$(PKG)'"
	GOPROXY=$(GOPROXY) $(PKG)

.PHONY: gogenerate
gogenerate:
	GOPROXY=$(GOPROXY) $(GO) generate ./...

.PHONY: gobuild
gobuild:
	@echo " :: Building releases into ./bin"
	mkdir -p $(BINDIR)
	GOPROXY=$(GOPROXY) CGO_ENABLED=$(CGO_ENABLED) $(GO) build -o $(BINDIR)/ -ldflags "$(LDFLAGS)" $(GO_EXTRA_FLAGS) $(PKGDIR)/...

.PHONY: docker-build-push
docker-build-push:
	@echo "Building and push docker image"
	docker buildx build --platform linux/amd64,linux/arm64 -t "jaredallard/localizer:latest" --push .

.PHONY: fmt
fmt:
	@echo " :: Running goimports"
	find  . -path ./vendor -prune -o -type f -name '*.go' -print | xargs ./scripts/gobin.sh golang.org/x/tools/cmd/goimports -w
	@echo " :: Running shfmt"
	./scripts/gobin.sh mvdan.cc/sh/v3/cmd/shfmt -l -w -s .
