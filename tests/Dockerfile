FROM docker.io/golang:1.18

ARG DEBIAN_FRONTEND=noninteractive

RUN apt update \
 && apt install -y git make gcc iputils-ping \
 && rm -rf /var/lib/apt/lists/*
RUN go install golang.zx2c4.com/wireguard@0.0.20220316

COPY entrypoint.sh /

WORKDIR /app

ENTRYPOINT [ "/entrypoint.sh" ]