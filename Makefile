VERSION=`git describe --tags --dirty --always`

GOFLAGS=-ldflags "-X main.version=${VERSION}" -gcflags=all=-trimpath=$(PWD) -asmflags=all=-trimpath=$(PWD)

build:
	GOARCH=amd64 go build ${GOFLAGS} -o wesher-amd64 ${OPTS}
	GOARCH=arm go build ${GOFLAGS} -o wesher-arm ${OPTS}
	GOARCH=arm64 go build ${GOFLAGS} -o wesher-arm64 ${OPTS}
	sha256sum wesher-* > wesher.sha256sums

e2e:
	tests/e2e.sh