VERSION := v1.0.0
NAME := names
BUILDSTRING := $(shell git log --pretty=format:'%h' -n 1)
VERSIONSTRING := $(NAME) version $(VERSION)+$(BUILDSTRING)

BUILDDATE := $(shell date -u -Iseconds)
OUTPUT = dist/$(NAME)-$(shell dpkg --print-architecture)-$(shell uname -s | awk '{print tolower($$0)}')

LDFLAGS := "-X \"main.VERSIONSTRING=$(VERSIONSTRING)\" -X \"main.BUILDDATE=$(BUILDDATE)\" -X \"main.VERSION=$(VERSION)\""

.PHONY: build
build:
	@mkdir -p dist/
	cd app; go build -o ../$(OUTPUT) -ldflags=$(LDFLAGS)
	cd -

.PHONY: clean
clean:
	rm -rf dist/

.PHONY: tag
tag:
	git tag $(VERSION)
	git push origin --tags

.PHONY: upx
upx:
	cd dist/ && upx names-amd64-linux

.PHONY: build_release
build_release: clean
	cd app; gox -arch="amd64" -os="windows darwin linux" -output="../dist/$(NAME)-{{.Arch}}-{{.OS}}" -ldflags=$(LDFLAGS)
