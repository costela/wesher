# stick to non-alpine so we can reuse build from travis host (avoid libc compat issues)
FROM golang:1.12

ARG DEBIAN_FRONTEND=noninteractive

RUN apt update \
 && apt install -y git make gcc \
 && rm -rf /var/lib/apt/lists/*
RUN go get -d golang.zx2c4.com/wireguard \
 && cd /go/src/golang.zx2c4.com/wireguard \
 && rm donotuseon_linux.go \
 && make install

ENV WG_I_PREFER_BUGGY_USERSPACE_TO_POLISHED_KMOD=1

COPY entrypoint.sh /

WORKDIR /app

ENTRYPOINT [ "/entrypoint.sh" ]