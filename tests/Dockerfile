# stick to non-alpine so we can reuse build from travis host (avoid libc compat issues)
FROM golang:1.12

RUN apk update && apk add git make gcc
RUN go get -d golang.zx2c4.com/wireguard \
 && cd /go/src/golang.zx2c4.com/wireguard \
 && rm donotuseon_linux.go \
 && make install

ENV WG_I_PREFER_BUGGY_USERSPACE_TO_POLISHED_KMOD=1

COPY entrypoint.sh /

WORKDIR /app

ENTRYPOINT [ "/entrypoint.sh" ]