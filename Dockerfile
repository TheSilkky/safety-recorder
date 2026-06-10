FROM golang:1.26-alpine@sha256:f23e8b227fb4493eabe03bede4d5a32d04092da71962f1fb79b5f7d1e6c2a17f AS builder

WORKDIR /src

RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/proofline-server ./cmd/api

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11

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
