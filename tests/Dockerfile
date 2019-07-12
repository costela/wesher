# pushed as costela/wesher-test to speed up testing

FROM golang:1.12-alpine

RUN apk update && apk add git make gcc
RUN go get -d golang.zx2c4.com/wireguard \
 && cd /go/src/golang.zx2c4.com/wireguard \
 && rm donotuseon_linux.go \
 && make install

ENV WG_I_PREFER_BUGGY_USERSPACE_TO_POLISHED_KMOD=1

COPY entrypoint.sh /

WORKDIR /app

ENTRYPOINT [ "/entrypoint.sh" ]