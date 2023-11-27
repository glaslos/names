VERSION := v1.3.0
NAME := names
BUILDSTRING := $(shell git log --pretty=format:'%h' -n 1)
VERSIONSTRING := $(NAME) version $(VERSION)+$(BUILDSTRING)

BUILDDATE := $(shell date -u -Iseconds)
OUTPUT = dist/$(NAME)-$(shell uname -m)-$(shell uname -s | awk '{print tolower($$0)}')
OUTPUT_ARM = dist/$(NAME)-arm64-$(shell uname -s | awk '{print tolower($$0)}')

LDFLAGS := "-X \"main.VERSIONSTRING=$(VERSIONSTRING)\" -X \"main.BUILDDATE=$(BUILDDATE)\" -X \"main.VERSION=$(VERSION)\""

.PHONY: build
build:
	@mkdir -p dist/
	cd app; go build -o ../$(OUTPUT) -ldflags=$(LDFLAGS)

.PHONY: build-arm
build-arm:
	@mkdir -p dist/
	cd app; env GOOS=linux GOARCH=arm64 go build -o ../$(OUTPUT_ARM) -ldflags=$(LDFLAGS)

.PHONY: clean
clean:
	rm -rf dist/

.PHONY: tag
tag:
	git tag $(VERSION)
	git push origin --tags

.PHONY: upx
upx:
	cd dist/ && upx *

.PHONY: build_release
build_release: clean
	cd app; gox -arch="amd64" -os="windows darwin linux" -output="../dist/$(NAME)-{{.Arch}}-{{.OS}}" -ldflags=$(LDFLAGS)

run:
	go build -o names app/app.go; sudo ./names; rm names

test:
	go test ./...
