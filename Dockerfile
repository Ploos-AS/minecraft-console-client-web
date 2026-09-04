# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine3.22 AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/mcc-web ./cmd/mcc-web

FROM alpine:3.22
ARG VERSION=dev
ARG REVISION=unknown
ARG CREATED=unknown
LABEL org.opencontainers.image.title="Minecraft Console Client Web" \
      org.opencontainers.image.description="Self-hosted WebAdmin for Minecraft Console Client" \
      org.opencontainers.image.url="https://github.com/Ploos-AS/minecraft-console-client-web" \
      org.opencontainers.image.source="https://github.com/Ploos-AS/minecraft-console-client-web" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.created="${CREATED}"
RUN apk add --no-cache ca-certificates \
 && addgroup -g 1000 -S mccweb \
 && adduser -u 1000 -S -D -H -G mccweb mccweb
COPY --from=build /out/mcc-web /usr/local/bin/mcc-web
USER 1000:1000
EXPOSE 8080
ENV MCC_WEB_LISTEN=:8080 \
    MCC_WS_URL=ws://mcc:8043/
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/api/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/mcc-web"]
