FROM golang:1.26-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 AS builder

WORKDIR /src

RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/proofline-server ./cmd/api

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S proofline \
	&& adduser -S -G proofline -h /nonexistent -s /sbin/nologin proofline \
	&& mkdir -p /var/lib/proofline /etc/proofline \
	&& chown -R proofline:proofline /var/lib/proofline

COPY --from=builder /out/proofline-server /usr/local/bin/proofline-server
COPY docker-default-config.toml /etc/proofline/proofline.toml

USER proofline
WORKDIR /var/lib/proofline
VOLUME ["/var/lib/proofline"]
EXPOSE 8080 8081

ENTRYPOINT ["proofline-server", "--config", "/etc/proofline/proofline.toml"]
