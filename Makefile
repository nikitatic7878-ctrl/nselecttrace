BINARY   := nselecttrace
PKG      := ./cmd/nselecttrace
VERSION  ?= 0.1.0
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE     := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -s -w \
	-X nselecttrace/internal/buildinfo.Version=$(VERSION) \
	-X nselecttrace/internal/buildinfo.Commit=$(COMMIT) \
	-X nselecttrace/internal/buildinfo.Date=$(DATE)

.PHONY: build run test vet fmt fmt-check clean cross release

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

run:
	go run $(PKG)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

cross:
	GOOS=linux   GOARCH=amd64 go build -trimpath -o dist/$(BINARY)-linux-amd64     $(PKG)
	GOOS=linux   GOARCH=arm64 go build -trimpath -o dist/$(BINARY)-linux-arm64     $(PKG)
	GOOS=darwin  GOARCH=amd64 go build -trimpath -o dist/$(BINARY)-darwin-amd64    $(PKG)
	GOOS=darwin  GOARCH=arm64 go build -trimpath -o dist/$(BINARY)-darwin-arm64    $(PKG)
	GOOS=windows GOARCH=amd64 go build -trimpath -o dist/$(BINARY)-windows-amd64.exe $(PKG)
	GOOS=windows GOARCH=arm64 go build -trimpath -o dist/$(BINARY)-windows-arm64.exe $(PKG)

# release builds the same six targets with the version metadata stamped in,
# so a local build matches what the release workflow produces.
release:
	GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64       $(PKG)
	GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64       $(PKG)
	GOOS=darwin  GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64      $(PKG)
	GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64      $(PKG)
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe $(PKG)
	GOOS=windows GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-arm64.exe $(PKG)

clean:
	rm -rf $(BINARY) $(BINARY).exe dist
