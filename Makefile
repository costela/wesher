VERSION := $(shell git describe --tags --dirty --always)

GOFLAGS := -ldflags "-X main.version=$(VERSION)" -gcflags=all=-trimpath=$(PWD) -asmflags=all=-trimpath=$(PWD)

GOARCHES := $(shell go env GOARCH)

build:
	$(foreach GOARCH,$(GOARCHES),GOARCH=$(GOARCH) go build ${GOFLAGS} -o wesher$(if $(filter-out $(GOARCH), $(GOARCHES)),-$(GOARCH));)

release: build
	sha256sum wesher-* > wesher.sha256sums

e2e:
	tests/e2e.sh

clean:
	rm -f wesher wesher-* wesher.sha256sums

.PHONY: build release e2e clean