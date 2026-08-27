# syntax=docker/dockerfile:1

FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
  -o /out/ipxe-manager ./cmd/ipxe-manager

FROM debian:bookworm-slim
RUN apt-get update \
  && apt-get install -y --no-install-recommends \
       ca-certificates curl \
       xorriso dosfstools mtools \
       build-essential liblzma-dev git \
       syslinux isolinux \
  && rm -rf /var/lib/apt/lists/* \
  && git clone --depth 1 https://github.com/ipxe/ipxe.git /opt/ipxe \
  && mkdir -p /opt/ipxe/src/config/local \
  && printf '%s\n' \
       '#define DOWNLOAD_PROTO_FILE' \
       '#define DOWNLOAD_PROTO_HTTPS' \
       '#define IMAGE_TRUST_CMD' \
       '#define REBOOT_CMD' \
       '#define PING_CMD' \
       '#define CONSOLE_CMD' \
       '#define IMAGE_EFI' \
       '#define SANBOOT_CMD' \
       > /opt/ipxe/src/config/local/general.h

COPY --from=build /out/ipxe-manager /usr/local/bin/ipxe-manager
COPY assets /opt/ipxe-manager/assets

ENV IPXE_ASSETS=/opt/ipxe-manager/assets \
    IPXE_DATA=/data \
    IPXE_LISTEN=:8081 \
    IPXE_SRC=/opt/ipxe/src

EXPOSE 8081
VOLUME ["/data"]
WORKDIR /data
ENTRYPOINT ["/usr/local/bin/ipxe-manager"]
