VERSION=`git describe --tags --dirty --always`

LDFLAGS=-ldflags "-X main.version=${VERSION}"

build:
	go build ${LDFLAGS} ${OPTS} -o wesher
	sha256sum wesher > wesher.sha256