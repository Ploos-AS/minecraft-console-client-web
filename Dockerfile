# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine3.22 AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/mcc-web ./cmd/mcc-web

FROM alpine:3.22
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
