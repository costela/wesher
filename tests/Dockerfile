FROM golang:1.18

ARG DEBIAN_FRONTEND=noninteractive

RUN apt update \
 && apt install -y git make gcc \
 && rm -rf /var/lib/apt/lists/*
RUN go get -d golang.zx2c4.com/wireguard \
 && cd /go/src/golang.zx2c4.com/wireguard \
 && make install

COPY entrypoint.sh /

WORKDIR /app

ENTRYPOINT [ "/entrypoint.sh" ]