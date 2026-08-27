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
  && apt-get install -y --no-install-recommends ca-certificates curl xorriso \
  && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/ipxe-manager /usr/local/bin/ipxe-manager
COPY assets /opt/ipxe-manager/assets
ENV IPXE_ASSETS=/opt/ipxe-manager/assets \
    IPXE_DATA=/data \
    IPXE_LISTEN=:8081
EXPOSE 8081
VOLUME ["/data"]
WORKDIR /data
ENTRYPOINT ["/usr/local/bin/ipxe-manager"]
