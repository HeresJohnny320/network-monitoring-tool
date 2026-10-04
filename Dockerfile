# Multi-arch image: docker build -t network-monitor .
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH TARGETVARIANT VERSION=docker
RUN GOARM=${TARGETVARIANT#v} CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/network_monitor_tool .

FROM alpine:3.22
ARG TARGETARCH
# iputils for ping, real traceroute (busybox's needs raw sockets). The Ookla CLI is a static binary.
RUN apk add --no-cache iputils traceroute ca-certificates tzdata \
 && case "$TARGETARCH" in \
      amd64) a=x86_64 ;; arm64) a=aarch64 ;; arm) a=armhf ;; 386) a=i386 ;; \
      *) echo "unsupported arch $TARGETARCH" && exit 1 ;; \
    esac \
 && wget -qO- "https://install.speedtest.net/app/cli/ookla-speedtest-1.2.0-linux-$a.tgz" | tar -xz -C /usr/local/bin speedtest
COPY --from=build /out/network_monitor_tool /usr/local/bin/network_monitor_tool
ENV NETMON_DATA_DIR=/data
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["network_monitor_tool", "-no-browser"]
